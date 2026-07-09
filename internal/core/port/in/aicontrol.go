package in

import "momo-shell/internal/core/domain"

// AIControlUseCase is the driving port for the AI-facing MCP surface
// (design doc 17 §4). This is the read-only slice: control/connect/run
// methods (RequestControl/ConnectHost/RunCommand/…) are added to this
// interface by the phase that implements them (B2/B3/C4) rather than
// stubbed now against domain types that don't yet exist -- mirroring how
// resolve.Verdict grows field-by-field
// (internal/core/service/aicontrol/resolve/resolve.go). clientID is the
// authenticated connection's principal, injected by the IPC adapter.
type AIControlUseCase interface {
	// ListSessions returns every open session's AI-facing view (state +
	// control-delegation flag).
	ListSessions(clientID string) ([]domain.SessionView, error)
	// ListHosts returns every saved host's name/labels only -- credentials
	// are never in this response, by construction (domain.HostRef).
	ListHosts(clientID string) ([]domain.HostRef, error)
	// ReadScrollback returns a masked slice of sessionID's scrollback
	// starting after sinceSeq.
	ReadScrollback(clientID, sessionID string, sinceSeq uint64) (domain.MaskedChunk, error)

	// ConnectHost resolves hostName against saved hosts and opens a fresh
	// SSH session against it, blocking for a human grant decision first
	// (design doc 17 §5.2, path A -- every connect_host call is a fresh
	// delegation). Credentials never cross this boundary: the caller only
	// ever supplies a name.
	ConnectHost(clientID, hostName string) (domain.SessionView, error)

	// RequestControl delegates an already-open session to clientID under
	// scope, blocking for a human grant decision. A session already
	// delegated to a different client is rejected outright (no prompt); a
	// repeat request from the same client is idempotent (returns the
	// existing Delegation).
	RequestControl(clientID, sessionID string, scope domain.ControlScope) (domain.Delegation, error)
	// ReleaseControl voluntarily gives up clientID's delegation over
	// sessionID. It does not touch any command the session may be running
	// -- unlike KillControl, this is the AI's own voluntary release, not
	// the local user's forced stop (see in.AIApprovalUseCase.KillControl,
	// doc 21's kill-switch decisions).
	ReleaseControl(clientID, sessionID string) error

	// RunCommand gates command through the 3-layer safety pipeline (doc 17
	// §6, doc 18 C4): requires an active delegation for sessionID by
	// clientID, statically resolves risk, auto-runs low-risk/certain
	// commands and blocks the rest for human approval, then injects.
	RunCommand(clientID, sessionID, command string) (domain.CommandHandle, error)
}
