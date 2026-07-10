package in

import "momo-shell/internal/core/domain"

// AuditUseCase is the driving port for E5's frontend-only audit-panel
// facade -- the seam audit_events.go (aicontrol) anticipated: a query/replay
// read path over the AI-control decision trail (doc 17 §10, doc 18 E3),
// deliberately separate from AIControlUseCase/MCPServerCallbacks since it has
// no AI-facing caller at all. audit.Service already satisfies this
// interface as-is; nothing in the audit core package changed shape to add
// it, only to add LoadOutputSegments (E5b).
type AuditUseCase interface {
	// Query returns sessionID's audit events in id-ascending (insertion,
	// i.e. replay) order, or every session's events if sessionID is "".
	Query(sessionID string) ([]domain.AuditEvent, error)

	// LoadOutputSegments returns auditID's captured original output (E3-b),
	// masked by default and split at secret-scanner hit boundaries so the
	// panel can render [REDACTED:type] markers with per-item unmask (E5b).
	// Returns (nil, nil) if no output was ever captured or attached, or if
	// retention already expired it.
	LoadOutputSegments(auditID int64) ([]domain.AuditOutputSegment, error)
}
