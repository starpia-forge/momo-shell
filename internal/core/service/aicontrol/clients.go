package aicontrol

import "momo-shell/internal/core/domain"

// ListMCPClients implements in.AIApprovalUseCase, the local user's side of
// A8's client-management UI. Pure pass-through to the repository -- no
// business logic (mirrors share.Service.ListClients).
func (s *Service) ListMCPClients() ([]domain.MCPClient, error) {
	return s.clients.List()
}

// RevokeMCPClient implements in.AIApprovalUseCase. This is a soft revoke
// (domain.MCPClient.Revoked): the row is kept for audit (E3) and an
// already-authenticated connection is not force-closed -- AuthClient
// rejects the token starting from the next auth attempt.
func (s *Service) RevokeMCPClient(clientID string) error {
	return s.clients.Revoke(clientID)
}
