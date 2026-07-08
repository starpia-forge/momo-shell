package in

import (
	"context"
	"errors"
)

// MCPServerCallbacks is the narrow driving port the mcpipc adapter calls
// for connection authentication -- separate from AIControlUseCase (the
// tool surface), mirroring how ShareServerCallbacks is separate from
// ShareUseCase. Dispatch (routing MCP requests to AIControlUseCase) is
// added in A6, once the mcp protocol types it needs exist.
type MCPServerCallbacks interface {
	// HandlePair blocks (up to the service's approval timeout) for the
	// local user's approve/deny decision -- no PIN (doc 20 D1: the S3
	// transport SID-gate already proves same-user, so pairing is human
	// approval alone). clientName is shown to the user, not otherwise
	// trusted. ctx is the inbound connection's context -- if the caller
	// disconnects before the approval decision arrives, HandlePair returns
	// without issuing a token instead of completing a pairing for an
	// absent peer.
	HandlePair(ctx context.Context, clientName string) (token string, err error)
	// AuthClient validates token and returns the internal clientID it
	// authorizes (the principal passed to AIControlUseCase methods).
	AuthClient(token string) (clientID string, err error)
}

// ErrUnauthorized is returned by AuthClient for an invalid or revoked
// token. Pairing failures (denial, timeout) reuse ShareServerCallbacks'
// existing sentinels (ErrPairDenied, ErrPairTimeout in share.go) -- MCP
// pairing is the same human-approval flow, so it shares the same
// indistinguishable-failure semantics rather than redefining them. There
// is no MCP equivalent of ErrPinMismatch/ErrLockedOut (doc 20 D1: no PIN).
var ErrUnauthorized = errors.New("mcp: invalid or revoked token")
