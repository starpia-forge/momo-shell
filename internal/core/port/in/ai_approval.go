package in

// AIApprovalUseCase is the driving port for the local user's approve/deny
// decisions on pending AI-control requests -- connect_host/control grants,
// run_command, and now MCP client pairing (RespondPairing, mirroring
// ShareUseCase.RespondPairing -- same requester/responder split as share,
// no PIN per doc 20 D1); secret-input approval joins this interface once
// its phase lands (D3). Separate from AIControlUseCase (the AI-facing side
// that raises and waits on the request) and from MCPServerCallbacks (the
// untrusted IPC adapter's pairing-request side) for the same reason
// ShareServerCallbacks is separate from ShareUseCase: requester and
// responder are different trust levels.
type AIApprovalUseCase interface {
	RespondConnectApproval(requestID string, approve bool) error
	RespondControlApproval(requestID string, approve bool) error
	RespondCommandApproval(requestID string, approve bool) error
	// RespondPairing answers a pending mcp:pair-request raised by an
	// incoming MCP client's pair frame (HandlePair).
	RespondPairing(requestID string, approve bool) error
}
