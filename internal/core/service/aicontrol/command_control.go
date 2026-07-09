package aicontrol

import (
	"errors"

	"momo-shell/internal/core/domain"
)

// ErrNoActiveCommand is returned by CancelCommand/BackgroundCommand when
// sessionID has no in-flight command handle, or its prior command already
// reached a terminal state (done/background).
var ErrNoActiveCommand = errors.New("aicontrol: session has no in-flight command")

// CancelCommand implements in.AIControlUseCase (doc 18 D2, doc 21's
// kill-switch decisions): interrupt sessionID's in-flight command with
// Ctrl-C while keeping the shell alive. Mirrors ResetShell's ownership guard
// and its choice not to touch the lifecycle FSM directly -- the
// shell-integration Observer (lifecycle.go's OnCommandEnd) drives
// running/tui -> done with the real exit code once the interrupt lands, the
// same way KillControl's best-effort Ctrl-C does for a borrowed session.
func (s *Service) CancelCommand(clientID, sessionID string) (domain.CommandHandle, error) {
	s.mu.Lock()
	deleg, ok := s.delegations[sessionID]
	if !ok {
		s.mu.Unlock()
		return domain.CommandHandle{}, ErrNotDelegated
	}
	if deleg.ClientID != clientID {
		s.mu.Unlock()
		return domain.CommandHandle{}, ErrNotYourDelegation
	}
	h, ok := s.commands[sessionID]
	if !ok || h.State.IsTerminal() {
		s.mu.Unlock()
		return domain.CommandHandle{}, ErrNoActiveCommand
	}
	snapshot := *h
	s.mu.Unlock()

	if err := s.sessions.Write(sessionID, []byte{0x03}); err != nil {
		return domain.CommandHandle{}, err
	}
	return snapshot, nil
}

// BackgroundCommand implements in.AIControlUseCase (doc 18 D2): suspend
// sessionID's in-flight command with Ctrl-Z and resume it detached via `bg`,
// then transition the lifecycle handle to CmdBackground. Unlike
// CancelCommand, this drives the FSM itself (no shell-integration signal
// distinguishes "backgrounded" from other alt-screen/exit events), so it
// publishes the new state the same way RunCommand publishes the initial
// running state.
func (s *Service) BackgroundCommand(clientID, sessionID string) (domain.CommandHandle, error) {
	s.mu.Lock()
	deleg, ok := s.delegations[sessionID]
	if !ok {
		s.mu.Unlock()
		return domain.CommandHandle{}, ErrNotDelegated
	}
	if deleg.ClientID != clientID {
		s.mu.Unlock()
		return domain.CommandHandle{}, ErrNotYourDelegation
	}
	if h, ok := s.commands[sessionID]; !ok || h.State.IsTerminal() {
		s.mu.Unlock()
		return domain.CommandHandle{}, ErrNoActiveCommand
	}
	s.mu.Unlock()

	// Write outside the lock (run.go's convention -- injection is I/O, not a
	// map mutation).
	if err := s.sessions.Write(sessionID, []byte{0x1a}); err != nil {
		return domain.CommandHandle{}, err
	}
	if err := s.sessions.Write(sessionID, []byte("bg"+lineTerminator)); err != nil {
		return domain.CommandHandle{}, err
	}

	s.mu.Lock()
	h, ok := s.commands[sessionID]
	if !ok || h.State.IsTerminal() {
		// Raced with a concurrent terminal transition (e.g. OnCommandEnd)
		// between the guard above and now -- the Ctrl-Z/bg bytes were still
		// sent (best-effort), but there's no handle left to move.
		s.mu.Unlock()
		return domain.CommandHandle{}, ErrNoActiveCommand
	}
	if err := h.TransitionTo(domain.CmdBackground); err != nil {
		s.mu.Unlock()
		return domain.CommandHandle{}, err
	}
	snapshot := *h
	s.mu.Unlock()

	s.publishCommandState(snapshot)
	return snapshot, nil
}
