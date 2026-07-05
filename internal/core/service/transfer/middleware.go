package transfer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// Service implements session.OutputMiddleware structurally (Attach/OnOutput/
// Detach) without importing the session package -- these two packages stay
// decoupled the same way transfer.ShellAccess keeps transfer from importing
// session for the reverse direction.

// zmodemPhase is a session's ZMODEM detection/transfer state.
type zmodemPhase string

const (
	zmodemIdle      zmodemPhase = "idle"
	zmodemAwaitSend zmodemPhase = "awaiting-send" // detected remote rz waiting; need StartZmodemSend
	zmodemActive    zmodemPhase = "active"
	// zmodemDraining follows every active transfer's end (success, failure,
	// or cancel): the remote rz/sz can keep streaming trailing protocol
	// bytes for a moment after we've stopped reading them as a transfer, and
	// rendering those to the terminal (which OnOutput would otherwise do
	// once phase is back to idle) means they get echoed and interpreted as
	// shell input -- see drainQuietPeriod/drainMaxDuration.
	zmodemDraining zmodemPhase = "draining"
)

// drainQuietPeriod/drainMaxDuration bound the draining phase: it ends as
// soon as drainQuietPeriod has passed with no further diverted output, but
// never later than drainMaxDuration after it began (a remote that's still
// actively spewing bad data shouldn't hold the shell hostage indefinitely).
const (
	drainQuietPeriod  = 500 * time.Millisecond
	drainMaxDuration  = 3 * time.Second
	drainPollInterval = 100 * time.Millisecond
)

// transferStallTimeout/transferStallPollInterval bound an active transfer
// with a last-resort watchdog: if no progress at all is made for
// transferStallTimeout, the transfer is forced to give up and the terminal
// is restored, regardless of what the ZMODEM engine goroutine is doing.
// This is set well above the engine's own read-timeout/retry budget
// (worst case ~100s: maxIORetries * ioReadTimeout) so it never preempts the
// engine's own bounded recovery attempts -- it exists purely to guarantee
// the terminal is never stuck indefinitely by a failure mode the engine's
// own bounds don't cover (observed in practice: FT-12's permanent freeze).
// var, not const, so tests can shrink them instead of waiting 120s for real.
var (
	transferStallTimeout      = 120 * time.Second
	transferStallPollInterval = 5 * time.Second
)

// zmodemState is one SSH session's ZMODEM detection state, guarded by its
// own mutex so OnOutput (pump goroutine) and StartZmodemSend/CancelZmodem
// (Wails-call goroutines) never race.
type zmodemState struct {
	mu       sync.Mutex
	detector signatureDetector
	phase    zmodemPhase
	conduit  *conduit
	cancel   context.CancelFunc
	taskID   string
	// pendingZRINIT holds the just-detected ZRINIT bytes while awaiting
	// StartZmodemSend. rz keeps re-announcing while it waits, but its retry
	// interval can be many seconds -- feeding this straight into the fresh
	// conduit lets our sender's negotiation see it immediately instead of
	// idling for however long the next retry takes.
	pendingZRINIT []byte
	// lastActivity is touched by every OnOutput call while draining, and
	// read by that drain's watcher goroutine to decide when it's been quiet
	// long enough to end.
	lastActivity time.Time
	// lastProgress is touched at transfer start and on every progress
	// callback while active, and read by the stall watchdog to decide when
	// a transfer has made no forward progress for too long.
	lastProgress time.Time
	// drainGen is bumped every time a new drain (or a new active transfer)
	// starts, so a stale watcher goroutine from a previous drain can tell
	// it's no longer the current one and exit instead of clobbering state.
	drainGen int
}

// ErrNoZmodemSession is returned by StartZmodemSend/CancelZmodem for a
// session ID the middleware never attached (local sessions, or unknown IDs).
var ErrNoZmodemSession = errors.New("transfer: no zmodem session")

// ErrNoZmodemPending is returned by StartZmodemSend when the session isn't
// currently waiting on one (no rz detected, or already transferring).
var ErrNoZmodemPending = errors.New("transfer: no pending zmodem upload request")

// zmodemPayload is published on transfer:zmodem:{sessionId}.
type zmodemPayload struct {
	Direction string `json:"direction"` // "upload" | "download"
	Phase     string `json:"phase"`     // "detected" | "active" | "done" | "failed" | "canceled"
	TaskID    string `json:"taskId,omitempty"`
}

func (s *Service) Attach(sessionID string, kind domain.SessionKind) {
	if kind != domain.KindSSH || s.zmodem == nil {
		return
	}
	s.zmu.Lock()
	s.zstates[sessionID] = &zmodemState{phase: zmodemIdle}
	s.zmu.Unlock()
}

func (s *Service) Detach(sessionID string) {
	s.zmu.Lock()
	st := s.zstates[sessionID]
	delete(s.zstates, sessionID)
	s.zmu.Unlock()
	if st == nil {
		return
	}
	st.mu.Lock()
	if st.cancel != nil {
		st.cancel()
	}
	if st.conduit != nil {
		st.conduit.Close()
	}
	st.mu.Unlock()
}

// OnOutput is called from the session's pump goroutine for every raw output
// chunk. It must never block indefinitely: while idle/awaiting, it only
// scans a bounded tail buffer; while active, conduit.push applies the same
// backpressure the pump already relies on elsewhere (see conduit.go).
func (s *Service) OnOutput(sessionID string, chunk []byte) []byte {
	s.zmu.Lock()
	st := s.zstates[sessionID]
	s.zmu.Unlock()
	if st == nil {
		return chunk
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	if st.phase == zmodemActive {
		st.conduit.push(chunk)
		return nil
	}
	if st.phase == zmodemDraining {
		st.lastActivity = time.Now()
		// Scan even while draining: a new transfer starting on the heels of
		// the last one's trailing bytes must not be missed just because it
		// arrived inside the drain's few-hundred-millisecond grace window
		// (previously, draining unconditionally discarded output without
		// scanning it at all). The bytes themselves are still discarded
		// either way -- draining's whole purpose -- only the detection
		// itself must not be skipped.
		_, direction, rest := st.detector.scan(chunk)
		if direction != "" {
			st.drainGen++ // supersede the now-superseded drain's watcher goroutine
			s.handleDetectionLocked(sessionID, st, direction, rest)
		}
		return nil
	}

	pass, direction, rest := st.detector.scan(chunk)
	if direction != "" {
		s.handleDetectionLocked(sessionID, st, direction, rest)
	}
	return pass
}

// handleDetectionLocked reacts to a freshly-detected signature: begins the
// download transfer immediately, or arms zmodemAwaitSend for an upload
// (rz keeps re-announcing ZRINIT every few seconds while it waits, so the
// freshest one is kept for StartZmodemSend rather than idling for the next
// retry). Caller must hold st.mu.
func (s *Service) handleDetectionLocked(sessionID string, st *zmodemState, direction string, rest []byte) {
	switch direction {
	case "download":
		s.beginTransferLocked(sessionID, st, "download", nil, rest)
	case "upload":
		wasAlreadyWaiting := st.phase == zmodemAwaitSend
		st.phase = zmodemAwaitSend
		st.pendingZRINIT = append([]byte{}, rest...)
		if !wasAlreadyWaiting {
			s.publishZmodem(sessionID, "upload", "detected", "")
		}
	}
}

// StartZmodemSend answers a pending rz-upload detection with the files the
// user picked or dropped.
func (s *Service) StartZmodemSend(sessionID string, localPaths []string) (string, error) {
	s.zmu.Lock()
	st := s.zstates[sessionID]
	s.zmu.Unlock()
	if st == nil {
		return "", ErrNoZmodemSession
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if st.phase != zmodemAwaitSend {
		return "", ErrNoZmodemPending
	}
	initial := st.pendingZRINIT
	st.pendingZRINIT = nil
	s.beginTransferLocked(sessionID, st, "upload", localPaths, initial)
	return st.taskID, nil
}

// CancelZmodem aborts any in-flight ZMODEM transfer (or a pending
// awaiting-send prompt) on a session.
func (s *Service) CancelZmodem(sessionID string) error {
	s.zmu.Lock()
	st := s.zstates[sessionID]
	s.zmu.Unlock()
	if st == nil {
		return ErrNoZmodemSession
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	if st.phase == zmodemActive {
		// Tell the remote rz/sz to give up immediately instead of sitting
		// on its own internal timeout, and unblock the engine's blocking
		// read right away (it won't otherwise notice ctx was canceled
		// until its next I/O deadline).
		if s.zmodem != nil {
			_ = s.shell.WriteRaw(sessionID, s.zmodem.CancelBytes())
		}
		if st.conduit != nil {
			st.conduit.Close()
		}
	}
	if st.cancel != nil {
		st.cancel()
	}
	if st.phase == zmodemAwaitSend {
		st.phase = zmodemIdle
		s.publishZmodem(sessionID, "upload", "canceled", "")
	} else if st.phase == zmodemDraining {
		// Cancel during the drain grace period should feel instant rather
		// than waiting out drainQuietPeriod/drainMaxDuration.
		st.phase = zmodemIdle
		st.drainGen++
		_ = s.shell.SetInputBlocked(sessionID, false)
	}
	return nil
}

// beginTransferLocked starts running the ZMODEM engine over a fresh
// conduit. Caller must hold st.mu.
func (s *Service) beginTransferLocked(sessionID string, st *zmodemState, direction string, localPaths []string, initial []byte) {
	ctx, cancel := context.WithCancel(context.Background())
	cd := newConduit(func(p []byte) error { return s.shell.WriteRaw(sessionID, p) })

	st.phase = zmodemActive
	st.conduit = cd
	st.cancel = cancel
	st.taskID = uuid.NewString()
	st.lastProgress = time.Now()
	st.drainGen++
	taskID := st.taskID
	debugf("session %s: phase -> active (%s, task %s)", sessionID, direction, taskID)

	_ = s.shell.SetInputBlocked(sessionID, true)
	s.publishZmodem(sessionID, direction, "active", taskID)

	if len(initial) > 0 {
		cd.push(initial)
	}

	go s.runZmodem(ctx, sessionID, st, cd, direction, localPaths, taskID)
	go s.watchTransferStall(sessionID, st, direction, taskID)
}

// watchTransferStall is the last-resort backstop for a transfer that never
// returns from the ZMODEM engine (see transferStallTimeout): once triggered,
// it nudges the remote to give up, cancels the engine's context, and closes
// the conduit, then forces the session straight into the drain phase so the
// terminal is restored even if the engine goroutine itself never unwinds.
// If runZmodem's own goroutine does eventually return (the common case --
// canceling ctx makes waitForHeaderBudget's loop exit on its next
// iteration), beginDrainLocked simply runs again, which is harmless.
func (s *Service) watchTransferStall(sessionID string, st *zmodemState, direction, taskID string) {
	ticker := time.NewTicker(transferStallPollInterval)
	defer ticker.Stop()
	for range ticker.C {
		st.mu.Lock()
		if st.taskID != taskID || st.phase != zmodemActive {
			st.mu.Unlock()
			return // this transfer already ended, one way or another
		}
		if time.Since(st.lastProgress) < transferStallTimeout {
			st.mu.Unlock()
			continue
		}
		cancel := st.cancel
		debugf("session %s: stall watchdog firing for task %s (no progress for %s)", sessionID, taskID, transferStallTimeout)
		if s.zmodem != nil {
			_ = s.shell.WriteRaw(sessionID, s.zmodem.CancelBytes())
		}
		if cancel != nil {
			cancel()
		}
		if st.conduit != nil {
			st.conduit.Close()
		}
		s.beginDrainLocked(sessionID, st)
		st.mu.Unlock()
		s.publishZmodem(sessionID, direction, "failed", taskID)
		return
	}
}

func (s *Service) runZmodem(ctx context.Context, sessionID string, st *zmodemState, cd *conduit, direction string, localPaths []string, taskID string) {
	progress := func(p out.TransferProgress) {
		st.mu.Lock()
		st.lastProgress = time.Now()
		st.mu.Unlock()
		s.publishProgressRaw(taskID, p)
	}

	var err error
	if direction == "download" {
		_, err = s.zmodem.Receive(ctx, cd, s.downloadDir, progress)
	} else {
		err = s.zmodem.Send(ctx, cd, localPaths, progress)
	}

	phase := "done"
	if err != nil {
		if ctx.Err() != nil {
			phase = "canceled"
		} else {
			phase = "failed"
			// A protocol failure (not a user cancel, which already sent
			// this from CancelZmodem) may leave the remote mid-stream;
			// tell it to give up rather than let it keep streaming into
			// the drain below on its own timeout.
			if s.zmodem != nil {
				_ = s.shell.WriteRaw(sessionID, s.zmodem.CancelBytes())
			}
		}
	}

	cd.Close()

	st.mu.Lock()
	st.conduit = nil
	st.cancel = nil
	s.beginDrainLocked(sessionID, st)
	st.mu.Unlock()

	s.publishZmodem(sessionID, direction, phase, taskID)
}

// beginDrainLocked transitions a just-finished transfer into a short grace
// period where diverted output is still discarded rather than rendered,
// absorbing whatever trailing protocol bytes the remote rz/sz is still
// mid-flight sending (which would otherwise leak to the terminal and get
// interpreted as shell input the instant input unblocks). Caller must hold
// st.mu; see zmodemDraining.
func (s *Service) beginDrainLocked(sessionID string, st *zmodemState) {
	st.phase = zmodemDraining
	st.lastActivity = time.Now()
	st.drainGen++
	gen := st.drainGen
	deadline := time.Now().Add(drainMaxDuration)
	debugf("session %s: phase -> draining (gen %d)", sessionID, gen)

	go func() {
		ticker := time.NewTicker(drainPollInterval)
		defer ticker.Stop()
		for range ticker.C {
			st.mu.Lock()
			if st.phase != zmodemDraining || st.drainGen != gen {
				st.mu.Unlock()
				return // superseded by a cancel, a new transfer, or Detach
			}
			quiet := time.Since(st.lastActivity) >= drainQuietPeriod
			expired := time.Now().After(deadline)
			if quiet || expired {
				st.phase = zmodemIdle
				st.mu.Unlock()
				debugf("session %s: phase -> idle (drain gen %d ended, quiet=%v expired=%v)", sessionID, gen, quiet, expired)
				_ = s.shell.SetInputBlocked(sessionID, false)
				return
			}
			st.mu.Unlock()
		}
	}()
}

// defaultDownloadDir resolves "~/Downloads" for when Deps.DownloadDir is
// left unset (the normal case; tests inject a temp dir instead).
func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}

func (s *Service) publishZmodem(sessionID, direction, phase, taskID string) {
	if s.pub == nil {
		return
	}
	s.pub.Publish(out.TopicTransferZmodem(sessionID), zmodemPayload{Direction: direction, Phase: phase, TaskID: taskID})
}

func (s *Service) publishProgressRaw(taskID string, p out.TransferProgress) {
	if s.pub == nil {
		return
	}
	s.pub.Publish(out.TopicTransferProgress(taskID), progressPayload{
		Bytes: p.Bytes,
		Total: p.Total,
		State: string(in.TaskRunning),
		File:  p.File,
	})
}
