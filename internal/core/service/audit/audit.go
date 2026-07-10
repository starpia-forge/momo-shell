// Package audit implements the AI-control decision audit trail (design doc
// 17 §10, doc 18 E3): append every connect/command/control decision
// aicontrol makes, and query it back in insertion order for replay (US-5).
package audit

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

type Service struct {
	repo out.AuditRepository
}

func New(repo out.AuditRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Record(e domain.AuditEvent) error {
	return s.repo.Append(e)
}

// Query returns sessionID's audit events in id-ascending (insertion) order
// -- that ordering is itself the replay sequence (US-5); there is no
// separate Replay method until output-interleaved or state-reconstruction
// replay (E3-b/E5) gives one distinct behavior.
func (s *Service) Query(sessionID string) ([]domain.AuditEvent, error) {
	return s.repo.List(sessionID)
}
