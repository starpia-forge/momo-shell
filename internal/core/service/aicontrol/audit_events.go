package aicontrol

import (
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/service/aicontrol/resolve"
)

// AuditRecorder is the narrow slice of audit.Service this package needs --
// a consumer-defined interface (CommandResolver/OutputMasker's pattern) so
// audit.Service is a peer core service, not infrastructure. An in.Audit-
// UseCase in-port would be the driver side for an inbound adapter (the E5
// Wails audit-panel facade), which has no caller yet -- defining it now
// would be exactly the speculative stub resolve.go:26-30 already warns
// against.
type AuditRecorder interface {
	Record(e domain.AuditEvent) error
}

// recordAudit appends e to the audit trail (design doc 17 §10, doc 18 E3).
// A nil s.audit (no Audit dep wired -- every existing test's Deps) makes
// this a no-op, mirroring publishDelegation's nil-pub guard
// (delegation_events.go). Append errors are swallowed: aicontrol has no
// logger, and returning the error up would fail command/connect/control
// flows over an audit-write failure. A silently dropped audit row is a
// known limitation, revisited when E3-b adds output capture.
func (s *Service) recordAudit(e domain.AuditEvent) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Record(e)
}

// recordConnectAudit records a ConnectHost decision. sessionID is "" for a
// denied/timed-out connect (no session was ever created); hostName is
// carried in Target since a connect event has no command of its own.
func (s *Service) recordConnectAudit(clientID, sessionID, hostName, approver, decision string) {
	s.recordAudit(domain.AuditEvent{
		Timestamp: time.Now(),
		ClientID:  clientID,
		SessionID: sessionID,
		Kind:      domain.AuditKindConnect,
		Target:    hostName,
		Approver:  approver,
		Decision:  decision,
	})
}

// recordCommandAudit records a RunCommand decision.
func (s *Service) recordCommandAudit(clientID, sessionID, command string, v resolve.Verdict, approver, decision string) {
	s.recordAudit(domain.AuditEvent{
		Timestamp:   time.Now(),
		ClientID:    clientID,
		SessionID:   sessionID,
		Kind:        domain.AuditKindCommand,
		OriginalCmd: command,
		GuardedCmd:  v.GuardedCmd,
		Resolve: domain.ResolveSummary{
			Risk:      string(v.Risk),
			Uncertain: v.Uncertain,
			Reasons:   v.Reasons,
		},
		Approver: approver,
		Decision: decision,
	})
}

// recordControlAudit records a RequestControl/ReleaseControl/KillControl
// decision.
func (s *Service) recordControlAudit(clientID, sessionID, approver, decision string) {
	s.recordAudit(domain.AuditEvent{
		Timestamp: time.Now(),
		ClientID:  clientID,
		SessionID: sessionID,
		Kind:      domain.AuditKindControl,
		Approver:  approver,
		Decision:  decision,
	})
}
