// Package audit implements the AI-control decision audit trail (design doc
// 17 §10, doc 18 E3): append every connect/command/control decision
// aicontrol makes, and query it back in insertion order for replay (US-5).
// E3-b adds the original-output side channel (AttachOutput/LoadOutput):
// encrypted locally, retention-bounded (PurgeOutputsBefore), and reachable
// only by explicit audit id -- never through Query, so the AI-facing replay
// path can't surface it.
package audit

import (
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// outputRetention bounds how long a captured command's output ciphertext is
// kept before PurgeOutputsBefore nulls it; the decision row itself is never
// deleted. A const for now -- E3-b's scope is the data layer, not a settings
// UI to make this configurable.
const outputRetention = 30 * 24 * time.Hour

type Service struct {
	repo   out.AuditRepository
	cipher *cipherBox
}

func New(repo out.AuditRepository, secrets out.SecretStore) *Service {
	return &Service{repo: repo, cipher: newCipherBox(secrets)}
}

// Record returns the new row's id so a caller (aicontrol's capture tap) can
// later attach output to this exact event via AttachOutput.
func (s *Service) Record(e domain.AuditEvent) (int64, error) {
	return s.repo.Append(e)
}

// Query returns sessionID's audit events in id-ascending (insertion) order
// -- that ordering is itself the replay sequence (US-5); there is no
// separate Replay method until output-interleaved or state-reconstruction
// replay (E5) gives one distinct behavior. Never carries the E3-b output
// capture -- domain.AuditEvent has no such field, and AuditRepository.List
// doesn't select output_ref.
func (s *Service) Query(sessionID string) ([]domain.AuditEvent, error) {
	return s.repo.List(sessionID)
}

// AttachOutput encrypts plaintext and attaches it to auditID's row (E3-b).
// Called from aicontrol's capture tap once a command's output is sealed.
func (s *Service) AttachOutput(auditID int64, plaintext []byte) error {
	ciphertext, err := s.cipher.seal(plaintext)
	if err != nil {
		return err
	}
	return s.repo.UpdateOutputRef(auditID, ciphertext)
}

// LoadOutput decrypts and returns auditID's attached output, or (nil, nil)
// if none is attached (no capture, or retention already expired it). Not
// reachable from the AI-facing MCP surface -- only E5's audit panel and
// tests call this.
func (s *Service) LoadOutput(auditID int64) ([]byte, error) {
	ciphertext, err := s.repo.LoadOutput(auditID)
	if err != nil || ciphertext == nil {
		return nil, err
	}
	return s.cipher.open(ciphertext)
}

// PurgeOutputsBefore nulls every output capture older than outputRetention
// as of now, keeping the decision rows themselves. Intended to be called
// from the same periodic sweep that drives aicontrol.SweepExpired (main.go).
func (s *Service) PurgeOutputsBefore(now time.Time) error {
	_, err := s.repo.ClearOutputsBefore(now.Add(-outputRetention).Unix())
	return err
}
