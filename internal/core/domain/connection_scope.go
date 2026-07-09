package domain

import "time"

// ConnectionScope is a fan-out bulk grant over a named set of hosts (design
// doc 17 §5.2, FR-10): a human approves one batch request instead of
// approving each connect_host call individually. It carries no FSM (unlike
// Delegation) -- it's a standing authorization record, not a state machine.
// A Delegation created under a scope records the link via its ScopeID field.
type ConnectionScope struct {
	ID            string
	ClientID      string
	HostNames     map[string]bool // the granted host set
	MaxConcurrent int             // visible concurrent-session cap -- doc 17 §5.2: reject once exceeded
	GrantedAt     time.Time
}
