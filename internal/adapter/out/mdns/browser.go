package mdns

import (
	"context"
	"strings"
	"sync"

	"github.com/libp2p/zeroconf/v2"

	"momo-shell/internal/core/port/out"
)

// Browser implements out.PeerBrowser, watching the LAN for other
// momo-shell instances advertising _momo-share._tcp.
type Browser struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewBrowser() *Browser {
	return &Browser{}
}

var _ out.PeerBrowser = (*Browser)(nil)

// Start begins browsing in the background. onUpdate is called with the
// full current peer set on every mDNS change; zeroconf closes the entries
// channel itself once ctx is canceled, which ends the update goroutine.
func (b *Browser) Start(onUpdate func([]out.DiscoveredPeer)) error {
	ctx, cancel := context.WithCancel(context.Background())

	b.mu.Lock()
	b.cancel = cancel
	b.mu.Unlock()

	entries := make(chan *zeroconf.ServiceEntry, 16)

	go func() {
		peers := make(map[string]out.DiscoveredPeer)
		for entry := range entries {
			peer, ok := parseEntry(entry)
			if !ok {
				continue
			}
			peers[peer.InstanceID] = peer
			onUpdate(snapshotPeers(peers))
		}
	}()

	go func() {
		_ = zeroconf.Browse(ctx, serviceType, domain, entries)
	}()

	return nil
}

func (b *Browser) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
}

func snapshotPeers(peers map[string]out.DiscoveredPeer) []out.DiscoveredPeer {
	result := make([]out.DiscoveredPeer, 0, len(peers))
	for _, p := range peers {
		result = append(result, p)
	}
	return result
}

// parseEntry extracts a DiscoveredPeer from a raw mDNS service entry,
// reading the id/name TXT fields set by Announcer. Entries missing an id
// or an IPv4 address are ignored.
func parseEntry(entry *zeroconf.ServiceEntry) (out.DiscoveredPeer, bool) {
	var id, name string
	for _, kv := range entry.Text {
		if v, ok := strings.CutPrefix(kv, "id="); ok {
			id = v
		}
		if v, ok := strings.CutPrefix(kv, "name="); ok {
			name = v
		}
	}
	if id == "" || len(entry.AddrIPv4) == 0 {
		return out.DiscoveredPeer{}, false
	}
	return out.DiscoveredPeer{
		InstanceID: id,
		Name:       name,
		Address:    entry.AddrIPv4[0].String(),
		Port:       entry.Port,
	}, true
}
