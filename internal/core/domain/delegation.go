package domain

import (
	"fmt"
	"time"
)

// DelegationState is the control-plane state for one session's AI
// delegation -- an overlay separate from Session's transport-level State
// (design doc 17 §5.1).
type DelegationState string

const (
	DelegNone            DelegationState = "none"
	DelegActive          DelegationState = "delegated"
	DelegAwaitingApprove DelegationState = "awaiting_approval"
	DelegAwaitingSecret  DelegationState = "awaiting_secret"
	DelegTUIHandoff      DelegationState = "tui_handoff"
	DelegExpired         DelegationState = "expired"
)

// legalDelegationTransitions encodes the allowed state machine (design doc
// 17 §5.1). Every awaiting_*/tui_handoff substate can also transition
// straight to DelegNone -- a kill switch/revoke must reach delegation
// immediately regardless of what it's waiting on (US-1).
var legalDelegationTransitions = map[DelegationState]map[DelegationState]bool{
	DelegNone:            {DelegActive: true},
	DelegActive:          {DelegNone: true, DelegExpired: true, DelegAwaitingApprove: true, DelegAwaitingSecret: true, DelegTUIHandoff: true},
	DelegAwaitingApprove: {DelegActive: true, DelegNone: true},
	DelegAwaitingSecret:  {DelegActive: true, DelegNone: true},
	DelegTUIHandoff:      {DelegActive: true, DelegNone: true},
	DelegExpired:         {DelegNone: true},
}

// ControlScope constrains a delegation. Enforcement (path prefix checks,
// read-only gating) belongs to the consumer that owns the corresponding
// action (e.g. C4's command gate, D5's context-hop rescoping) -- this is
// data only, matching design doc 17 §5.1.
type ControlScope struct {
	HostOnly   bool
	PathPrefix string
	ReadOnly   bool
}

// Delegation is the control-plane entity attached to a session: which AI
// client currently holds control, under what scope, and its FSM state.
type Delegation struct {
	SessionID string
	ClientID  string
	State     DelegationState
	Scope     ControlScope
	GrantedAt time.Time
	LastActAt time.Time // last-activity timestamp; auto-expiry is computed from this
	ScopeID   string    // owning ConnectionScope (fan-out grant), "" if none
}

// NewDelegation creates a delegation in the initial None state -- the
// caller transitions it to Active once a human approves the initial grant.
func NewDelegation(sessionID, clientID string, scope ControlScope) *Delegation {
	return &Delegation{
		SessionID: sessionID,
		ClientID:  clientID,
		State:     DelegNone,
		Scope:     scope,
	}
}

// TransitionTo moves the delegation to next, rejecting illegal transitions.
func (d *Delegation) TransitionTo(next DelegationState) error {
	if !legalDelegationTransitions[d.State][next] {
		return fmt.Errorf("domain: invalid delegation state transition %s -> %s", d.State, next)
	}
	d.State = next
	return nil
}

// IsExpired reports whether the delegation has been inactive for at least
// timeout as of now. A zero LastActAt (never set) is never expired -- that's
// the caller's responsibility to have set on grant.
func (d *Delegation) IsExpired(now time.Time, timeout time.Duration) bool {
	if d.LastActAt.IsZero() {
		return false
	}
	return now.Sub(d.LastActAt) >= timeout
}
