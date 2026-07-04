package transfer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

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
	if st.cancel != nil {
		st.cancel()
	}
	if st.phase == zmodemAwaitSend {
		st.phase = zmodemIdle
		s.publishZmodem(sessionID, "upload", "canceled", "")
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

	cd.Close()
	_ = s.shell.SetInputBlocked(sessionID, false)

	st.mu.Lock()
	st.phase = zmodemIdle
	st.conduit = nil
	st.cancel = nil
	st.mu.Unlock()

	phase := "done"
	if err != nil {
		if ctx.Err() != nil {
			phase = "canceled"
		} else {
			phase = "failed"
		}
	}
	s.publishZmodem(sessionID, direction, phase, taskID)
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
