package domain

import "time"

// AuditKind categorizes an AuditEvent. Only the kinds this codebase
// currently emits are defined -- design doc 17 §10 lists more ("approval",
// "context_hop", "secret_input", "probe"), but "approval" folds into
// AuditEvent.Decision on a command event rather than getting its own row,
// and the rest belong to features that don't emit yet: context_hop is
// D5 (deferred), secret_input was D3 (dissolved), probe is C2 (not yet
// consumed by any resolve/run path). They land as new AuditKind values
// alongside the feature that starts producing them.
type AuditKind string

const (
	AuditKindConnect AuditKind = "connect"
	AuditKindCommand AuditKind = "command"
	AuditKindControl AuditKind = "control"
)

// ResolveSummary is the subset of resolve.Verdict worth persisting to the
// audit trail. Risk is a plain string, not resolve.RiskLevel -- domain
// stays free of service-package imports; aicontrol maps
// string(verdict.Risk) when building the event (it already does this for
// commandApprovalPayload).
type ResolveSummary struct {
	Risk      string
	Uncertain bool
	Reasons   []string
}

// AuditEvent is a single recorded AI-control decision (design doc 17 §10,
// doc 18 E3): who acted, what was decided, and why. Decision's vocabulary
// is kind-specific: command -> auto|approved|rejected|timeout; control ->
// granted|released|killed|rejected|timeout; connect -> auto|granted|
// rejected|timeout. Approver is "custodian" for every human-made decision
// (this app has exactly one local human, so there is no identity to model)
// and "" when no human was in the loop (auto-run).
type AuditEvent struct {
	ID          int64
	Timestamp   time.Time
	ClientID    string
	SessionID   string
	Kind        AuditKind
	Target      string // kind-specific subject, e.g. the host name for a connect event
	OriginalCmd string
	GuardedCmd  string
	Resolve     ResolveSummary
	Approver    string
	Decision    string
}
