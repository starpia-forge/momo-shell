package aicontrol

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
	"momo-shell/internal/core/service/aicontrol/resolve"
)

// lineTerminator submits an injected command the same way a real terminal
// does: xterm's onData emits "\r" for Enter (frontend/.../terminal-registry.ts),
// and session.Service.Write forwards bytes to the PTY/SSH stream unmodified
// -- not a POSIX text-file "\n".
const lineTerminator = "\r"

var commandApprovalTimeout = 60 * time.Second // var, not const -- test-shrinkable

var (
	ErrCommandDenied                  = errors.New("aicontrol: command denied")
	ErrCommandApprovalTimeout         = errors.New("aicontrol: command approval timed out")
	ErrNoPendingCommandApproval       = errors.New("aicontrol: no pending command approval request")
	ErrCommandApprovalAlreadyAnswered = errors.New("aicontrol: command approval request already answered")
)

// commandApprovalPayload is TopicMCPCommandApproval's payload, defined next
// to its publisher (house convention -- cf. connectApprovalPayload,
// controlApprovalPayload).
type commandApprovalPayload struct {
	RequestID  string   `json:"requestId"`
	ClientID   string   `json:"clientId"`
	SessionID  string   `json:"sessionId"`
	Command    string   `json:"command"` // original -- masking is E2's job
	Risk       string   `json:"risk"`
	Uncertain  bool     `json:"uncertain"`
	Reasons    []string `json:"reasons"`
	GuardedCmd string   `json:"guardedCmd"` // C5 shows original/guarded separately (FR-7)
}

// RunCommand implements in.AIControlUseCase: the run_command 3-gate pipeline
// (design doc 17 §6, doc 18 C4) -- authorize the delegation, statically
// resolve the command's risk, gate low-risk-and-certain commands straight
// through and everything else behind a human approval, then inject.
func (s *Service) RunCommand(clientID, sessionID, command string) (domain.CommandHandle, error) {
	s.mu.Lock()
	deleg, ok := s.delegations[sessionID]
	s.mu.Unlock()
	if !ok {
		return domain.CommandHandle{}, ErrNotDelegated
	}
	if deleg.ClientID != clientID {
		return domain.CommandHandle{}, ErrNotYourDelegation
	}

	shell, _, live := s.sessions.SessionShell(sessionID)
	if !live {
		return domain.CommandHandle{}, ErrSessionNotFound
	}

	verdict := s.resolver.Resolve(command, dialectFromShell(shell))

	toRun := command
	if verdict.GuardedCmd != "" {
		toRun = verdict.GuardedCmd
	}

	if !verdict.AutoRunnable() {
		approved, err := s.awaitCommandApproval(clientID, sessionID, command, verdict)
		if err != nil {
			return domain.CommandHandle{}, err
		}
		if !approved {
			return domain.CommandHandle{}, ErrCommandDenied
		}
	}

	if err := s.sessions.Write(sessionID, []byte(toRun+lineTerminator)); err != nil {
		return domain.CommandHandle{}, err
	}

	// Injection succeeded: bump delegation activity and arm the execution
	// lifecycle (D1) -- lifecycle.go's Observer callbacks drive it onward.
	h := domain.NewCommandHandle(sessionID, toRun)
	s.mu.Lock()
	if d, ok := s.delegations[sessionID]; ok {
		d.LastActAt = time.Now()
	}
	s.commands[sessionID] = h // a new command replaces any prior handle for this session
	snapshot := *h
	s.mu.Unlock()

	s.publishCommandState(snapshot) // stream the initial running state
	return snapshot, nil
}

// dialectFromShell maps a session's resolved shell (session.Service.
// SessionShell's shell string -- a full local shell path, or "" for SSH
// sessions since the remote shell is never recorded) to the dialect token
// resolve/shparse recognizes ("bash"/"sh"). Anything shparse doesn't
// recognize (PowerShell, cmd.exe, SSH's "") is passed through as-is --
// CommandResolver.Resolve degrades an unrecognized dialect to a
// conservative high-risk/uncertain verdict on its own (doc 17 §491:
// unsupported dialect -> forced approval), so no extra fallback is needed
// here.
func dialectFromShell(shell string) string {
	if shell == "" {
		return ""
	}
	base := strings.ToLower(filepath.Base(shell))
	switch {
	case strings.Contains(base, "bash"):
		return "bash"
	case base == "sh" || strings.Contains(base, "dash"):
		return "sh"
	default:
		return base
	}
}

// awaitCommandApproval publishes mcp:cmd-approval and blocks (up to
// commandApprovalTimeout) for the matching RespondCommandApproval call.
// Structurally identical to awaitControlApproval/awaitConnectApproval, a
// separate topic/payload so the frontend can render a command-approval
// dialog (resolve's risk/reasons/guarded form) rather than a connect/control
// one. Shares the same s.pending map -- requestIDs are UUIDs, no collision
// risk across request kinds.
func (s *Service) awaitCommandApproval(clientID, sessionID, command string, v resolve.Verdict) (bool, error) {
	requestID := uuid.NewString()
	respCh := make(chan bool, 1)

	s.mu.Lock()
	s.pending[requestID] = respCh
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, requestID)
		s.mu.Unlock()
	}()

	if s.pub != nil {
		s.pub.Publish(out.TopicMCPCommandApproval(), commandApprovalPayload{
			RequestID:  requestID,
			ClientID:   clientID,
			SessionID:  sessionID,
			Command:    command,
			Risk:       string(v.Risk),
			Uncertain:  v.Uncertain,
			Reasons:    v.Reasons,
			GuardedCmd: v.GuardedCmd,
		})
	}

	select {
	case approved := <-respCh:
		return approved, nil
	case <-time.After(commandApprovalTimeout):
		return false, ErrCommandApprovalTimeout
	}
}

// RespondCommandApproval implements in.AIApprovalUseCase, answering a
// pending mcp:cmd-approval request raised by awaitCommandApproval.
func (s *Service) RespondCommandApproval(requestID string, approve bool) error {
	s.mu.Lock()
	ch, ok := s.pending[requestID]
	s.mu.Unlock()
	if !ok {
		return ErrNoPendingCommandApproval
	}
	select {
	case ch <- approve:
		return nil
	default:
		return ErrCommandApprovalAlreadyAnswered
	}
}
