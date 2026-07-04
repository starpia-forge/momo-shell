package domain

import "fmt"

// SessionKind distinguishes the underlying terminal transport.
type SessionKind string

const (
	KindLocal SessionKind = "local"
)

// SessionState is the session lifecycle state.
type SessionState string

const (
	StateStarting SessionState = "starting"
	StateRunning  SessionState = "running"
	StateClosed   SessionState = "closed"
	StateError    SessionState = "error"
)

// legalTransitions encodes the allowed state machine; terminal states (Closed, Error) have none.
var legalTransitions = map[SessionState]map[SessionState]bool{
	StateStarting: {StateRunning: true, StateClosed: true, StateError: true},
	StateRunning:  {StateClosed: true, StateError: true},
	StateClosed:   {},
	StateError:    {},
}

// Session is the core entity representing one terminal (local shell or, in later phases, SSH).
type Session struct {
	ID    string
	Kind  SessionKind
	Shell string
	Cols  int
	Rows  int
	State SessionState
}

// NewSession creates a session in the initial Starting state.
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
	ID    string      `json:"id"`
	Kind  SessionKind `json:"kind"`
	Shell string      `json:"shell"`
	Cols  int         `json:"cols"`
	Rows  int         `json:"rows"`
}

// Info snapshots the session as a SessionInfo DTO.
func (s *Session) Info() SessionInfo {
	return SessionInfo{ID: s.ID, Kind: s.Kind, Shell: s.Shell, Cols: s.Cols, Rows: s.Rows}
}
