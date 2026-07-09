package aicontrol

import (
	"context"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// shellStateProbeTimeout bounds how long GetShellState waits for a probe
// reply. var, not const -- test-shrinkable (commandApprovalTimeout mirror).
var shellStateProbeTimeout = 5 * time.Second

// shellStateEnvNames are the "주요 env" (design doc 17 §5.1) GetShellState
// probes alongside PWD. var, not const, so the set is easy to tune later.
var shellStateEnvNames = []string{"HOME", "USER", "SHELL"}

// ShellStateReader is the narrow, core-defined interface GetShellState needs
// from shell-integration's variable probe -- consumer-side (SessionCreator's
// pattern). A main.go adapter flattens shellintegration.Service.Query's
// map[string]VarValue into this plain map[string]string so this package
// never imports shellintegration (read.go's OutputMasker import-avoidance
// convention).
type ShellStateReader interface {
	ReadVars(ctx context.Context, sessionID string, names []string) (map[string]string, error)
}

// shellStatePayload is TopicMCPShellState's payload, defined next to its
// publisher (house convention -- cf. connectApprovalPayload).
type shellStatePayload struct {
	SessionID string            `json:"sessionId"`
	Cwd       string            `json:"cwd"`
	Env       map[string]string `json:"env"`
}

// GetShellState implements in.AIControlUseCase: it probes sessionID's shell
// for its cwd ($PWD) and a fixed set of main env vars (design doc 17 §5.1,
// FR-3), publishing the result on mcp:shell-state for the frontend badge
// before returning it. On-demand, not continuously tracked -- calling this
// from a shell-integration Observer callback would deadlock the probe (it
// blocks on the same output-pump goroutine that would deliver the reply).
func (s *Service) GetShellState(clientID, sessionID string) (domain.ShellState, error) {
	s.mu.Lock()
	deleg, ok := s.delegations[sessionID]
	s.mu.Unlock()
	if !ok {
		return domain.ShellState{}, ErrNotDelegated
	}
	if deleg.ClientID != clientID {
		return domain.ShellState{}, ErrNotYourDelegation
	}

	ctx, cancel := context.WithTimeout(context.Background(), shellStateProbeTimeout)
	defer cancel()

	names := append([]string{"PWD"}, shellStateEnvNames...)
	vars, err := s.shellState.ReadVars(ctx, sessionID, names)
	if err != nil {
		return domain.ShellState{}, err
	}

	state := domain.ShellState{SessionID: sessionID, Cwd: vars["PWD"], Env: map[string]string{}}
	for _, name := range shellStateEnvNames {
		if v, ok := vars[name]; ok {
			state.Env[name] = v
		}
	}

	if s.pub != nil {
		s.pub.Publish(out.TopicMCPShellState(), shellStatePayload{
			SessionID: state.SessionID,
			Cwd:       state.Cwd,
			Env:       state.Env,
		})
	}

	return state, nil
}

// ResetShell implements in.AIControlUseCase: a clean reset for AI-caused
// shell contamination (design doc 17 §5.1: "reset_shell(오염 blast radius
// 대응)") -- interrupt any in-flight command (Ctrl-C, KillControl's cleanup
// byte) and return to the home directory. It does not revoke the delegation
// (unlike KillControl) and does not restore env vars the AI may have
// exported -- that needs a grant-time env snapshot, deferred to a later
// cycle.
func (s *Service) ResetShell(clientID, sessionID string) error {
	s.mu.Lock()
	deleg, ok := s.delegations[sessionID]
	s.mu.Unlock()
	if !ok {
		return ErrNotDelegated
	}
	if deleg.ClientID != clientID {
		return ErrNotYourDelegation
	}

	return s.sessions.Write(sessionID, []byte("\x03cd ~"+lineTerminator))
}
