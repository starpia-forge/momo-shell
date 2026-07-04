package domain

import "fmt"

// SessionKind distinguishes the underlying terminal transport.
type SessionKind string

const (
	KindLocal SessionKind = "local"
	KindSSH   SessionKind = "ssh"
)

// SessionState is the session lifecycle state.
type SessionState string

const (
	// StateConnecting is SSH-only: dialing, handshaking, and (if needed)
	// waiting on a host key confirmation. Local sessions skip it and start
	// at StateStarting since a local shell spawns synchronously.
	StateConnecting SessionState = "connecting"
	StateStarting   SessionState = "starting"
	StateRunning    SessionState = "running"
	StateClosed     SessionState = "closed"
	StateError      SessionState = "error"
)

// legalTransitions encodes the allowed state machine; terminal states (Closed, Error) have none.
var legalTransitions = map[SessionState]map[SessionState]bool{
	StateConnecting: {StateRunning: true, StateClosed: true, StateError: true},
	StateStarting:   {StateRunning: true, StateClosed: true, StateError: true},
	StateRunning:    {StateClosed: true, StateError: true},
	StateClosed:     {},
	StateError:      {},
}

// Session is the core entity representing one terminal (local shell or SSH).
type Session struct {
	ID     string
	Kind   SessionKind
	Shell  string // local only; empty for SSH
	HostID string // SSH only; empty for local
	Cols   int
	Rows   int
	State  SessionState
}

// NewSession creates a local session in the initial Starting state.
func NewSession(id string, kind SessionKind, shell string, cols, rows int) *Session {
	return &Session{
		ID:    id,
		Kind:  kind,
		Shell: shell,
		Cols:  cols,
		Rows:  rows,
		State: StateStarting,
	}
}

// NewSSHSession creates an SSH session in the initial Connecting state.
func NewSSHSession(id, hostID string, cols, rows int) *Session {
	return &Session{
		ID:     id,
		Kind:   KindSSH,
		HostID: hostID,
		Cols:   cols,
		Rows:   rows,
		State:  StateConnecting,
	}
}

// TransitionTo moves the session to next, rejecting illegal transitions.
func (s *Session) TransitionTo(next SessionState) error {
	if !legalTransitions[s.State][next] {
		return fmt.Errorf("domain: invalid session state transition %s -> %s", s.State, next)
	}
	s.State = next
	return nil
}

// SessionInfo is the read-only DTO handed back across the in-port boundary.
type SessionInfo struct {
	ID     string      `json:"id"`
	Kind   SessionKind `json:"kind"`
	Shell  string      `json:"shell,omitempty"`
	HostID string      `json:"hostId,omitempty"`
	Cols   int         `json:"cols"`
	Rows   int         `json:"rows"`
}

// Info snapshots the session as a SessionInfo DTO.
func (s *Session) Info() SessionInfo {
	return SessionInfo{ID: s.ID, Kind: s.Kind, Shell: s.Shell, HostID: s.HostID, Cols: s.Cols, Rows: s.Rows}
}
