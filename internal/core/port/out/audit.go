package out

import "momo-shell/internal/core/domain"

// AuditRepository persists AI-control decision audit events (design doc 17
// §10, doc 18 E3): the "last line of defense" trail of every connect/
// command/control decision aicontrol makes.
type AuditRepository interface {
	// Append returns the row's assigned id (E3-b links a later output
	// capture to this exact row via UpdateOutputRef).
	Append(e domain.AuditEvent) (int64, error)
	// List returns sessionID's events in insertion order (id ASC -- the
	// replay sequence, US-5). sessionID == "" returns every event. Never
	// selects output_ref -- the AI-facing replay path has no route to the
	// raw output capture.
	List(sessionID string) ([]domain.AuditEvent, error)
	// UpdateOutputRef attaches auditID's encrypted output capture (E3-b).
	UpdateOutputRef(auditID int64, ciphertext []byte) error
	// LoadOutput returns auditID's encrypted output capture, or (nil, nil)
	// if none is attached.
	LoadOutput(auditID int64) ([]byte, error)
	// ClearOutputsBefore nulls output_ref for every row older than
	// cutoffUnix that still has one attached, returning the count cleared.
	// The decision row itself is never removed.
	ClearOutputsBefore(cutoffUnix int64) (int64, error)
}
