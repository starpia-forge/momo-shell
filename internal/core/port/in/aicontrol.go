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
}
