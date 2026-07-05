package mdns

import (
	"fmt"
	"sync"

	"github.com/libp2p/zeroconf/v2"

	"momo-shell/internal/core/port/out"
)

// Announcer implements out.PeerAnnouncer.
type Announcer struct {
	mu     sync.Mutex
	server *zeroconf.Server
}

func NewAnnouncer() *Announcer {
	return &Announcer{}
}

var _ out.PeerAnnouncer = (*Announcer)(nil)

// Announce advertises this instance as _momo-share._tcp.local., carrying
// its persistent instance ID and display name in the TXT record (spec
// §2.1: "v=1", "id={instanceId}", "name={공유 이름}").
func (a *Announcer) Announce(instanceID, name string, port int) error {
	server, err := zeroconf.Register(instanceID, serviceType, domain, port, []string{
		"v=1",
		"id=" + instanceID,
		"name=" + name,
	}, nil)
	if err != nil {
		return fmt.Errorf("mdns: register: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.server = server
	return nil
}

func (a *Announcer) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server != nil {
		a.server.Shutdown()
		a.server = nil
	}
}
