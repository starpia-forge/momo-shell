package in

import "momo-shell/internal/core/domain"

// AIApprovalUseCase is the driving port for the local user's approve/deny
// decisions on pending AI-control requests -- connect_host/control grants,
// run_command, and now MCP client pairing (RespondPairing, mirroring
// ShareUseCase.RespondPairing -- same requester/responder split as share,
// no PIN per doc 20 D1). D3's secret-input approval dissolved without
// joining this interface (doc 22) -- echo-off detection is unobservable
// over SSH/ConPTY, and the human-co-resident GUI terminal model handles
// interactive secrets without an AI-mediated path. Separate from
// AIControlUseCase (the AI-facing side
// that raises and waits on the request) and from MCPServerCallbacks (the
// untrusted IPC adapter's pairing-request side) for the same reason
// ShareServerCallbacks is separate from ShareUseCase: requester and
// responder are different trust levels. Also carries client administration
// (list/revoke, A8) -- the local user's side of managing who has been
// paired, mirroring ShareUseCase bundling ListClients/RevokeClient alongside
// RespondPairing. And the kill switch (KillControl, B2) -- another local-user
// action, not an AI-facing one, so it belongs here rather than on
// AIControlUseCase.
type AIApprovalUseCase interface {
	RespondConnectApproval(requestID string, approve bool) error
	RespondControlApproval(requestID string, approve bool) error
	// RespondConnectionScopeApproval answers a pending
	// mcp:connection-scope-approval request raised by RequestConnectionScope
	// (fan-out batch grant, doc 17 §5.2/FR-10).
	RespondConnectionScopeApproval(requestID string, approve bool) error
	RespondCommandApproval(requestID string, approve bool) error
	// RespondPairing answers a pending mcp:pair-request raised by an
	// incoming MCP client's pair frame (HandlePair).
	RespondPairing(requestID string, approve bool) error

	// KillControl is the local user's emergency stop (US-1, doc 21): it
	// revokes sessionID's delegation immediately (the safety guarantee)
	// and best-effort cleans up the in-flight command by session origin --
	// closes an AI-created session entirely, or interrupts (Ctrl-C) a
	// borrowed user session while preserving the shell. Client-agnostic
	// and user-forced, unlike ReleaseControl's voluntary/client-scoped
	// release.
	KillControl(sessionID string) error

	// ListMCPClients returns every MCP client this instance has ever paired
	// with, including revoked ones (audit trail, E3).
	ListMCPClients() ([]domain.MCPClient, error)
	// RevokeMCPClient soft-revokes clientID: the row is kept but flagged so
	// the next AuthClient call rejects its token. An already-authenticated
	// connection is not force-closed by this call.
	RevokeMCPClient(clientID string) error

	// ListDelegations returns every currently active delegation, for the
	// frontend's control panel to seed its store on load (B5a) --
	// mcp:delegation only reports transitions from that point forward.
	ListDelegations() ([]domain.Delegation, error)

	// ListConnectionScopes returns every active fan-out grant with its live
	// active-delegation count against MaxConcurrent, for the control panel's
	// concurrent-session cap display (B5b, doc 18 §4).
	ListConnectionScopes() ([]domain.ConnectionScopeStatus, error)
}
