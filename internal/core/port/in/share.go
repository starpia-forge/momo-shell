package in

import (
	"context"
	"errors"
	"time"

	"momo-shell/internal/core/domain"
)

// ShareUseCase is the driving port for LAN host sharing, bound to the
// local Wails UI: both the provider role (share this instance's hosts
// with peers, M1) and the consumer role (discover and pair with peers
// sharing hosts back, M2).
type ShareUseCase interface {
	// EnableSharing turns on the LAN HTTPS server and mDNS advertisement,
	// generates a fresh pairing PIN, and marks hostIDs as shared. Calling
	// it again while already enabled regenerates the PIN and replaces the
	// shared host set without restarting the server.
	EnableSharing(hostIDs []string) (ShareStatus, error)
	DisableSharing() error
	Status() (ShareStatus, error)
	SetSharedHosts(hostIDs []string) error
	ListClients() ([]domain.ShareClient, error)
	RevokeClient(clientID string) error
	// RespondPairing answers a pending share:pair-request raised by an
	// incoming /pair call.
	RespondPairing(requestID string, approve bool) error

	// ListPeers returns every peer this instance knows about -- paired
	// peers (from storage) merged with currently-discovered-but-unpaired
	// peers (from mDNS), each annotated with online status and its cached
	// shared-host list.
	ListPeers() ([]PeerView, error)
	// PairWithPeer pairs with a peer discovered via mDNS, identified by
	// its mDNS instance ID (PeerView.ID for an unpaired entry).
	PairWithPeer(peerID string, pin string) error
	// AddPeerByAddress pairs with a peer reachable at address:port,
	// bypassing mDNS discovery -- the fallback for networks that block
	// multicast.
	AddPeerByAddress(address string, port int, pin string) error
	RemovePeer(peerID string) error
	// FetchSharedHosts force-refreshes and returns the cached host list
	// for an already-paired peer.
	FetchSharedHosts(peerID string) ([]domain.SharedHost, error)
	// ImportSharedHost saves peer.Hosts[index] as a new local Host
	// (Source: "shared:{peerID}"), so it appears in "내 호스트" like any
	// other saved host. The credential is not carried over -- the caller
	// (frontend) is responsible for a subsequent SetHostSecret if desired.
	ImportSharedHost(peerID string, index int) (domain.Host, error)
}

// PeerView is the consumer-side snapshot of one peer shown in the host
// sidebar's shared-hosts section.
type PeerView struct {
	ID         string
	Name       string
	Address    string
	Port       int
	Paired     bool
	Online     bool
	LastSyncAt *time.Time
	Hosts      []domain.SharedHost
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
	// ctx is the inbound HTTP request's context -- if the caller disconnects
	// before the approval decision arrives, HandlePair returns without
	// issuing a token instead of completing a pairing for an absent peer.
	HandlePair(ctx context.Context, pin, clientName, remoteAddr string) (token string, err error)
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
