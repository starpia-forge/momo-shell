package aicontrol

import "momo-shell/internal/core/domain"

// ListConnectionScopes returns every active fan-out grant (design doc 17
// §5.2) alongside its live active-delegation count against MaxConcurrent,
// for the control panel's "N/cap" display (B5b). Mirrors ListDelegations's
// snapshot-then-return shape (delegation_events.go). Scopes are never
// deleted (only overwritten per-client, connect.go's s.scopes doc comment),
// so this can return stale-looking entries whose ActiveCount has dropped to
// 0 -- that's accurate, not a bug: the grant is still standing authorization.
func (s *Service) ListConnectionScopes() ([]domain.ConnectionScopeStatus, error) {
	s.mu.Lock()
	scopes := make([]*domain.ConnectionScope, 0, len(s.scopes))
	for _, scope := range s.scopes {
		scopes = append(scopes, scope)
	}
	s.mu.Unlock()

	// liveDelegationCountForScope takes s.mu itself, so it must be called
	// only after the snapshot above has released it (connect.go's
	// checkConnectionScope follows the same lock-then-release-then-call
	// order).
	statuses := make([]domain.ConnectionScopeStatus, 0, len(scopes))
	for _, scope := range scopes {
		hostNames := make([]string, 0, len(scope.HostNames))
		for name := range scope.HostNames {
			hostNames = append(hostNames, name)
		}
		statuses = append(statuses, domain.ConnectionScopeStatus{
			ScopeID:       scope.ID,
			ClientID:      scope.ClientID,
			HostNames:     hostNames,
			MaxConcurrent: scope.MaxConcurrent,
			ActiveCount:   s.liveDelegationCountForScope(scope.ID),
		})
	}
	return statuses, nil
}
