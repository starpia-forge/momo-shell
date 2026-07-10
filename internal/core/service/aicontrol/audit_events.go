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
// AuditRecorder.Record returns the appended row's id so recordCommandAudit
// can hand it to the capture tap (E3-b: Begin(sessionID, auditID) links a
// later-sealed output capture to this exact row).
type AuditRecorder interface {
	Record(e domain.AuditEvent) (int64, error)
}

// recordAudit appends e to the audit trail (design doc 17 §10, doc 18 E3)
// and returns its id, or 0 if no row was recorded. A nil s.audit (no Audit
// dep wired -- every existing test's Deps) makes this a no-op, mirroring
// publishDelegation's nil-pub guard (delegation_events.go). Append errors
// are swallowed: aicontrol has no logger, and returning the error up would
// fail command/connect/control flows over an audit-write failure. A
// silently dropped audit row (id 0) also means E3-b's capture never begins
// for that command -- a known limitation shared with the original gap this
// comment used to describe.
func (s *Service) recordAudit(e domain.AuditEvent) int64 {
	if s.audit == nil {
		return 0
	}
	id, err := s.audit.Record(e)
	if err != nil {
		return 0
	}
	return id
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

// recordCommandAudit records a RunCommand decision and returns its audit id
// (0 if not recorded), which RunCommand hands to the capture tap's Begin
// for an executed command.
func (s *Service) recordCommandAudit(clientID, sessionID, command string, v resolve.Verdict, approver, decision string) int64 {
	return s.recordAudit(domain.AuditEvent{
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
