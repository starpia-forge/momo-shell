package aicontrol

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// ListSessions implements in.AIControlUseCase: every live session's
// AI-facing view, joining session.Service's lifecycle state (Snapshot)
// with this service's own delegation registry for the Controlled flag.
// clientID is accepted (matching in.AIControlUseCase) but not yet
// consulted -- reserved for the audit trail (E3), same as ReadScrollback.
func (s *Service) ListSessions(clientID string) ([]domain.SessionView, error) {
	live := s.sessions.Snapshot()

	s.mu.Lock()
	defer s.mu.Unlock()
	views := make([]domain.SessionView, 0, len(live))
	for _, sess := range live {
		_, controlled := s.delegations[sess.ID]
		views = append(views, domain.SessionView{
			ID:         sess.ID,
			Kind:       sess.Kind,
			HostID:     sess.HostID,
			State:      sess.State,
			Controlled: controlled,
		})
	}
	return views, nil
}

// ListHosts implements in.AIControlUseCase: every saved host, projected to
// domain.HostRef so credentials (address/username/auth/key) are excluded
// by construction -- the AI only ever learns a host's name (design doc 16
// §핵심원칙①).
func (s *Service) ListHosts(clientID string) ([]domain.HostRef, error) {
	hosts, err := s.hosts.List()
	if err != nil {
		return nil, err
	}
	refs := make([]domain.HostRef, 0, len(hosts))
	for _, h := range hosts {
		refs = append(refs, domain.HostRef{Name: h.Name, Labels: h.Labels})
	}
	return refs, nil
}

var _ in.AIControlUseCase = (*Service)(nil)
