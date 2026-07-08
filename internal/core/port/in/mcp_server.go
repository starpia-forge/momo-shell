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
	// HandlePair validates pin, then blocks (up to the service's approval
	// timeout) for the local user's approve/deny decision. clientName is
	// shown to the user, not otherwise trusted. ctx is the inbound
	// connection's context -- if the caller disconnects before the
	// approval decision arrives, HandlePair returns without issuing a
	// token instead of completing a pairing for an absent peer.
	HandlePair(ctx context.Context, pin, clientName string) (token string, err error)
	// AuthClient validates token and returns the internal clientID it
	// authorizes (the principal passed to AIControlUseCase methods).
	AuthClient(token string) (clientID string, err error)
}

// ErrUnauthorized is returned by AuthClient for an invalid or revoked
// token. Pairing failures (pin mismatch, denial, timeout, lockout) reuse
// ShareServerCallbacks' existing sentinels (ErrPinMismatch, ErrPairDenied,
// ErrPairTimeout, ErrLockedOut in share.go) -- MCP pairing is the same
// human-approval flow, so it shares the same indistinguishable-failure
// semantics rather than redefining them.
var ErrUnauthorized = errors.New("mcp: invalid or revoked token")
