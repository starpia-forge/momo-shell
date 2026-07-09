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
// MCP-facing approve/deny decisions (connect/control/command/connection-scope
// grants and client pairing), client administration (list/revoke), and the
// control kill switch (KillControl). It only passes calls through -- no
// business logic -- mirroring ShareService's
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

func (s *MCPApprovalService) RespondConnectionScopeApproval(requestID string, approve bool) error {
	return s.uc.RespondConnectionScopeApproval(requestID, approve)
}

func (s *MCPApprovalService) RespondCommandApproval(requestID string, approve bool) error {
	return s.uc.RespondCommandApproval(requestID, approve)
}

func (s *MCPApprovalService) RespondPairing(requestID string, approve bool) error {
	return s.uc.RespondPairing(requestID, approve)
}

func (s *MCPApprovalService) KillControl(sessionID string) error {
	return s.uc.KillControl(sessionID)
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

// DelegationDTO is the JSON-facing response DTO for an active delegation
// (B5a's control-panel snapshot). Unlike domain.Delegation, it omits
// State/Scope/LastActAt/ScopeID -- ListDelegations only ever returns active
// delegations (the panel doesn't need the FSM substate), and the panel's
// scope/expiry display is deferred to B5b.
type DelegationDTO struct {
	SessionID string `json:"sessionId"`
	ClientID  string `json:"clientId"`
	AICreated bool   `json:"aiCreated"`
	GrantedAt int64  `json:"grantedAt"`
}

func (s *MCPApprovalService) ListDelegations() ([]DelegationDTO, error) {
	delegations, err := s.uc.ListDelegations()
	if err != nil {
		return nil, err
	}
	dtos := make([]DelegationDTO, len(delegations))
	for i, d := range delegations {
		dtos[i] = delegationToDTO(d)
	}
	return dtos, nil
}

func delegationToDTO(d domain.Delegation) DelegationDTO {
	return DelegationDTO{SessionID: d.SessionID, ClientID: d.ClientID, AICreated: d.AICreated, GrantedAt: d.GrantedAt.Unix()}
}

// ConnectionScopeDTO is the JSON-facing response DTO for an active fan-out
// grant plus its live active-delegation count (B5b's concurrent-session cap
// display).
type ConnectionScopeDTO struct {
	ScopeID       string   `json:"scopeId"`
	ClientID      string   `json:"clientId"`
	HostNames     []string `json:"hostNames"`
	MaxConcurrent int      `json:"maxConcurrent"`
	ActiveCount   int      `json:"activeCount"`
}

func (s *MCPApprovalService) ListConnectionScopes() ([]ConnectionScopeDTO, error) {
	scopes, err := s.uc.ListConnectionScopes()
	if err != nil {
		return nil, err
	}
	dtos := make([]ConnectionScopeDTO, len(scopes))
	for i, sc := range scopes {
		dtos[i] = connectionScopeToDTO(sc)
	}
	return dtos, nil
}

func connectionScopeToDTO(sc domain.ConnectionScopeStatus) ConnectionScopeDTO {
	return ConnectionScopeDTO{
		ScopeID:       sc.ScopeID,
		ClientID:      sc.ClientID,
		HostNames:     sc.HostNames,
		MaxConcurrent: sc.MaxConcurrent,
		ActiveCount:   sc.ActiveCount,
	}
}

func mcpClientToDTO(c domain.MCPClient) MCPClientDTO {
	dto := MCPClientDTO{ID: c.ClientID, Name: c.Name, PairedAt: c.PairedAt.Unix(), Revoked: c.Revoked}
	if c.LastSeenAt != nil {
		t := c.LastSeenAt.Unix()
		dto.LastSeenAt = &t
	}
	return dto
}
