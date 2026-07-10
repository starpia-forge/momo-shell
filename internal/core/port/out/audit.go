package out

import "momo-shell/internal/core/domain"

// AuditRepository persists AI-control decision audit events (design doc 17
// §10, doc 18 E3): the "last line of defense" trail of every connect/
// command/control decision aicontrol makes.
type AuditRepository interface {
	Append(e domain.AuditEvent) error
	// List returns sessionID's events in insertion order (id ASC -- the
	// replay sequence, US-5). sessionID == "" returns every event.
	List(sessionID string) ([]domain.AuditEvent, error)
}
