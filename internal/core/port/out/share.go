package out

import (
	"context"

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
}
