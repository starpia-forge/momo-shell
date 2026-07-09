package wails

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// MCPClientDTO is the JSON-facing response DTO for a paired MCP client
// (A8's settings-page client list). Unlike ShareClientDTO it carries
// Revoked, since revocation is soft (domain.MCPClient's audit-trail
// comment) and the UI needs to distinguish a revoked row from a live one.
type MCPClientDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PairedAt   int64  `json:"pairedAt"`
	LastSeenAt *int64 `json:"lastSeenAt,omitempty"`
	Revoked    bool   `json:"revoked"`
}

// MCPApprovalService is the Wails-bound facade over in.AIApprovalUseCase's
// MCP-facing approve/deny decisions (connect/control/command grants and
// client pairing) plus client administration (list/revoke). It only passes
// calls through -- no business logic -- mirroring ShareService's
// RespondPairing/ListClients/RevokeClient convention.
type MCPApprovalService struct {
	uc in.AIApprovalUseCase
}

func NewMCPApprovalService(uc in.AIApprovalUseCase) *MCPApprovalService {
	return &MCPApprovalService{uc: uc}
}

func (s *MCPApprovalService) RespondConnectApproval(requestID string, approve bool) error {
	return s.uc.RespondConnectApproval(requestID, approve)
}

func (s *MCPApprovalService) RespondControlApproval(requestID string, approve bool) error {
	return s.uc.RespondControlApproval(requestID, approve)
}

func (s *MCPApprovalService) RespondCommandApproval(requestID string, approve bool) error {
	return s.uc.RespondCommandApproval(requestID, approve)
}

func (s *MCPApprovalService) RespondPairing(requestID string, approve bool) error {
	return s.uc.RespondPairing(requestID, approve)
}

func (s *MCPApprovalService) ListClients() ([]MCPClientDTO, error) {
	clients, err := s.uc.ListMCPClients()
	if err != nil {
		return nil, err
	}
	dtos := make([]MCPClientDTO, len(clients))
	for i, c := range clients {
		dtos[i] = mcpClientToDTO(c)
	}
	return dtos, nil
}

func (s *MCPApprovalService) RevokeClient(clientID string) error {
	return s.uc.RevokeMCPClient(clientID)
}

func mcpClientToDTO(c domain.MCPClient) MCPClientDTO {
	dto := MCPClientDTO{ID: c.ClientID, Name: c.Name, PairedAt: c.PairedAt.Unix(), Revoked: c.Revoked}
	if c.LastSeenAt != nil {
		t := c.LastSeenAt.Unix()
		dto.LastSeenAt = &t
	}
	return dto
}
