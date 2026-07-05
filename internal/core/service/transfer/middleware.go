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
		return nil
	}

	pass, direction, rest := st.detector.scan(chunk)
	if direction == "" {
		return pass
	}

	switch direction {
	case "download":
		s.beginTransferLocked(sessionID, st, "download", nil, rest)
	case "upload":
		wasAlreadyWaiting := st.phase == zmodemAwaitSend
		st.phase = zmodemAwaitSend
		// Keep the freshest ZRINIT -- rz re-announces periodically (often
		// every several seconds) while it waits, and feeding this straight
		// into StartZmodemSend's conduit means our sender doesn't have to
		// idle for the next retry to arrive.
		st.pendingZRINIT = append([]byte{}, rest...)
		if !wasAlreadyWaiting {
			s.publishZmodem(sessionID, "upload", "detected", "")
		}
	}
	return pass
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
	st.drainGen++
	taskID := st.taskID

	_ = s.shell.SetInputBlocked(sessionID, true)
	s.publishZmodem(sessionID, direction, "active", taskID)

	if len(initial) > 0 {
		cd.push(initial)
	}

	go s.runZmodem(ctx, sessionID, st, cd, direction, localPaths, taskID)
}

func (s *Service) runZmodem(ctx context.Context, sessionID string, st *zmodemState, cd *conduit, direction string, localPaths []string, taskID string) {
	progress := func(p out.TransferProgress) {
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
