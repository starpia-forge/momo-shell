package out

import (
	"context"
	"errors"

	"momo-shell/internal/core/domain"
)

// ShareServer is the driven port for the LAN HTTPS server that answers
// pairing and host-list requests from peers. Wired in after construction
// via the share service's SetServer, the same two-phase pattern as
// session.Service.SetMiddleware: the server needs a callback into the
// share service, and the share service needs to Start/Stop the server.
type ShareServer interface {
	// Start binds the configured listeners and begins serving. Returns the
	// bound port, which may differ from the preferred one if it was busy.
	Start() (port int, err error)
	Stop(ctx context.Context) error
}

// ShareClientRepository persists clients this instance has paired with in
// the provider role (i.e. issued a bearer token to). Tokens are stored
// hashed -- see core/service/share.
type ShareClientRepository interface {
	List() ([]domain.ShareClient, error)
	FindByTokenHash(hash string) (domain.ShareClient, bool, error)
	Save(c domain.ShareClient, tokenHash string) error
	Delete(id string) error
	TouchSeen(id string) error
}

// ShareSettings persists small provider-side configuration: this
// instance's identity and which host IDs are currently shared.
type ShareSettings interface {
	// InstanceID returns this instance's persistent identifier, generating
	// and storing one on first call.
	InstanceID() (string, error)
	SharedHostIDs() ([]string, error)
	SetSharedHostIDs(ids []string) error
	// DeviceName returns the custom device name override, or "" if unset
	// (the caller should fall back to os.Hostname()).
	DeviceName() (string, error)
	SetDeviceName(name string) error
}

// PeerRepository persists peers this instance has paired with in the
// consumer role, including their cached shared-host list.
type PeerRepository interface {
	List() ([]domain.Peer, error)
	Get(id string) (domain.Peer, error)
	Save(p domain.Peer) error
	Delete(id string) error
}

// PeerAnnouncer advertises this instance over mDNS while sharing is
// enabled, so other instances' PeerBrowser can discover it.
type PeerAnnouncer interface {
	Announce(instanceID, name string, port int) error
	Stop()
}

// DiscoveredPeer is one mDNS browse result -- a candidate peer visible on
// the LAN, not yet necessarily paired.
type DiscoveredPeer struct {
	InstanceID string
	Name       string
	Address    string
	Port       int
}

// PeerBrowser watches the LAN for other momo-shell instances advertising
// _momo-share._tcp. onUpdate is called with the full current set on every
// change (additions/removals/TTL expiry), not incrementally.
type PeerBrowser interface {
	Start(onUpdate func([]DiscoveredPeer)) error
	Stop()
}

// PeerInfo mirrors a peer's GET /info response.
type PeerInfo struct {
	Ver       int
	ID        string
	Name      string
	HostCount int
}

// ErrPeerUnauthorized is returned by PeerClient.FetchHosts when the peer
// has revoked this instance's token -- the caller should drop the pairing
// rather than merely mark the peer offline.
var ErrPeerUnauthorized = errors.New("share: peer rejected our token")

// ErrPeerCertMismatch is returned by any PeerClient method called with a
// non-empty certFP when the peer presents a different certificate than the
// one pinned at pairing time -- the peer may have been reinstalled or is
// being impersonated.
var ErrPeerCertMismatch = errors.New("share: peer certificate fingerprint changed since pairing")

// PeerClient is the driven port for calling another instance's LAN share
// API. certFP == "" means TOFU: accept whatever certificate the peer
// presents and return its fingerprint for the caller to persist; a
// non-empty certFP pins the connection to that exact certificate.
type PeerClient interface {
	Info(address string, port int, certFP string) (info PeerInfo, observedFP string, err error)
	Pair(address string, port int, certFP, pin, clientName string) (token, observedFP string, err error)
	// FetchHosts returns ErrPeerUnauthorized on a 401 response.
	FetchHosts(address string, port int, certFP, token string) ([]domain.SharedHost, error)
}
