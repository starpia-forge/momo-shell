package in

// AIApprovalUseCase is the driving port for the local user's approve/deny
// decisions on pending AI-control requests -- connect_host/control grants
// and now run_command; secret-input approval joins this interface once its
// phase lands (D3), mirroring ShareUseCase.RespondPairing. Separate from
// AIControlUseCase (the AI-facing side that raises and waits on the
// request) for the same reason ShareServerCallbacks is separate from
// ShareUseCase: requester and responder are different trust levels.
type AIApprovalUseCase interface {
	RespondConnectApproval(requestID string, approve bool) error
	RespondControlApproval(requestID string, approve bool) error
	RespondCommandApproval(requestID string, approve bool) error
}
