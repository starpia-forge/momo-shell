package wails

import "momo-shell/internal/core/port/in"

// MCPApprovalService is the Wails-bound facade over in.AIApprovalUseCase's
// MCP-facing approve/deny decisions (connect/control/command grants and
// client pairing). It only passes calls through -- no business logic --
// mirroring ShareService.RespondPairing's convention. The frontend
// dialogs that call these (A8) are not part of this cycle; this facade
// exists so A8 can start as a pure frontend change.
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
