package in

import (
	"errors"

	"momo-shell/internal/core/domain"
)

// ShareUseCase is the driving port for LAN host sharing, bound to the
// local Wails UI. M1 covers the provider role (share this instance's
// hosts with peers); the consumer role (discover and pair with peers
// sharing hosts back) is added in M2.
type ShareUseCase interface {
	// EnableSharing turns on the LAN HTTPS server and mDNS advertisement
	// (once wired in M2), generates a fresh pairing PIN, and marks hostIDs
	// as shared. Calling it again while already enabled regenerates the
	// PIN and replaces the shared host set without restarting the server.
	EnableSharing(hostIDs []string) (ShareStatus, error)
	DisableSharing() error
	Status() (ShareStatus, error)
	SetSharedHosts(hostIDs []string) error
	ListClients() ([]domain.ShareClient, error)
	RevokeClient(clientID string) error
	// RespondPairing answers a pending share:pair-request raised by an
	// incoming /pair call.
	RespondPairing(requestID string, approve bool) error
}

// ShareStatus is the provider-side snapshot shown in the share settings panel.
type ShareStatus struct {
	Enabled       bool
	PIN           string
	Port          int
	InstanceName  string
	SharedHostIDs []string
}

// ShareInfo mirrors the LAN API's GET /info response.
type ShareInfo struct {
	Ver       int
	ID        string
	Name      string
	HostCount int
}

// ShareServerCallbacks is the narrow driving port the sharehttp adapter
// calls into for each LAN API request. It is separate from ShareUseCase
// (bound to the local Wails UI) because a remote peer's request surface is
// much smaller and must never expose EnableSharing/RevokeClient/etc.
type ShareServerCallbacks interface {
	Info() ShareInfo
	// HandlePair validates pin, then blocks (up to the service's approval
	// timeout) for the local user's approve/deny decision via
	// RespondPairing. remoteAddr is shown to the user, not otherwise trusted.
	HandlePair(pin, clientName, remoteAddr string) (token string, err error)
	// HostsForToken validates token and returns the shared host list.
	HostsForToken(token string) ([]domain.SharedHost, error)
}

// Sentinel errors returned by ShareServerCallbacks.HandlePair/HostsForToken,
// mapped to HTTP status codes by the sharehttp adapter. PIN mismatch,
// denial, and timeout are intentionally indistinguishable to the caller.
var (
	ErrPinMismatch       = errors.New("share: pin mismatch")
	ErrLockedOut         = errors.New("share: too many failed pairing attempts")
	ErrPairDenied        = errors.New("share: pairing request denied")
	ErrPairTimeout       = errors.New("share: pairing request timed out")
	ErrShareUnauthorized = errors.New("share: invalid or revoked token")
)
