package share

import (
	"errors"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

func withShrunkSyncInterval(t *testing.T, d time.Duration) {
	t.Helper()
	original := peerSyncInterval
	peerSyncInterval = d
	t.Cleanup(func() { peerSyncInterval = original })
}

func TestListPeers_MergesDiscoveredAndPaired(t *testing.T) {
	ts := newTestService()
	if err := ts.svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ts.svc.Close()

	if err := ts.peers.Save(domain.Peer{ID: "paired-1", Name: "Starpia-PC", Address: "10.0.1.5", Port: 47800, PairedAt: time.Now()}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}
	ts.browser.push([]out.DiscoveredPeer{
		{InstanceID: "paired-1", Name: "Starpia-PC", Address: "10.0.1.5", Port: 47800}, // online, already paired
		{InstanceID: "unpaired-1", Name: "kim-laptop", Address: "10.0.1.9", Port: 47800},
	})

	views, err := ts.svc.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers() error = %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("ListPeers() = %+v, want 2 entries", views)
	}

	byID := map[string]in.PeerView{}
	for _, v := range views {
		byID[v.ID] = v
	}

	if p := byID["paired-1"]; !p.Paired || !p.Online {
		t.Fatalf("paired-1 = %+v, want Paired=true Online=true", p)
	}
	if p := byID["unpaired-1"]; p.Paired || !p.Online {
		t.Fatalf("unpaired-1 = %+v, want Paired=false Online=true", p)
	}
}

// TestListPeers_ExcludesSelfDiscoveredPeer guards against the E2E-observed
// self-discovery quirk: an instance's own mDNS advertisement is
// indistinguishable on the wire from any other peer's, so onDiscoveryUpdate
// must filter it out by instance ID rather than surfacing it as a
// discoverable "peer" of itself.
func TestListPeers_ExcludesSelfDiscoveredPeer(t *testing.T) {
	ts := newTestService()
	if err := ts.svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ts.svc.Close()

	selfID, err := ts.settings.InstanceID()
	if err != nil {
		t.Fatalf("InstanceID() error = %v", err)
	}
	ts.browser.push([]out.DiscoveredPeer{
		{InstanceID: selfID, Name: "self", Address: "10.0.1.5", Port: 47800},
		{InstanceID: "unpaired-1", Name: "kim-laptop", Address: "10.0.1.9", Port: 47800},
	})

	views, err := ts.svc.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers() error = %v", err)
	}
	if len(views) != 1 || views[0].ID != "unpaired-1" {
		t.Fatalf("ListPeers() = %+v, want only unpaired-1 (self excluded)", views)
	}
}

func TestListPeers_PairedPeerOfflineWhenNotDiscovered(t *testing.T) {
	ts := newTestService()
	if err := ts.peers.Save(domain.Peer{ID: "paired-1", Name: "Starpia-PC", Address: "10.0.1.5", Port: 47800, PairedAt: time.Now()}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}

	views, err := ts.svc.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers() error = %v", err)
	}
	if len(views) != 1 || views[0].Online {
		t.Fatalf("ListPeers() = %+v, want 1 offline entry", views)
	}
}

func TestPairWithPeer_NotDiscoveredReturnsError(t *testing.T) {
	ts := newTestService()
	if err := ts.svc.PairWithPeer("ghost", "123456"); !errors.Is(err, ErrPeerNotDiscovered) {
		t.Fatalf("err = %v, want ErrPeerNotDiscovered", err)
	}
}

func TestPairWithPeer_Success(t *testing.T) {
	ts := newTestService()
	if err := ts.svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ts.svc.Close()

	ts.browser.push([]out.DiscoveredPeer{{InstanceID: "peer-1", Name: "kim-laptop", Address: "10.0.1.9", Port: 47800}})

	ts.peerClient.infoFunc = func(address string, port int, certFP string) (out.PeerInfo, string, error) {
		if address != "10.0.1.9" || port != 47800 {
			t.Fatalf("Info() called with unexpected address:port = %s:%d", address, port)
		}
		return out.PeerInfo{Ver: 1, ID: "peer-1", Name: "kim-laptop", HostCount: 2}, "fp-abc", nil
	}
	ts.peerClient.pairFunc = func(address string, port int, certFP, pin, clientName string) (string, string, error) {
		if pin != "654321" {
			t.Fatalf("Pair() pin = %q, want 654321", pin)
		}
		return "token-xyz", "fp-abc", nil
	}

	if err := ts.svc.PairWithPeer("peer-1", "654321"); err != nil {
		t.Fatalf("PairWithPeer() error = %v", err)
	}

	peer, err := ts.peers.Get("peer-1")
	if err != nil {
		t.Fatalf("peers.Get() error = %v", err)
	}
	if peer.Name != "kim-laptop" || peer.CertFingerprint != "fp-abc" {
		t.Fatalf("stored peer = %+v", peer)
	}
	if !ts.secrets.has(peerTokenRef("peer-1")) {
		t.Fatal("expected token stored in secret store")
	}
}

func TestAddPeerByAddress_PairsWithoutDiscovery(t *testing.T) {
	ts := newTestService()

	ts.peerClient.infoFunc = func(address string, port int, certFP string) (out.PeerInfo, string, error) {
		return out.PeerInfo{ID: "peer-2", Name: "office-mac"}, "fp-direct", nil
	}
	ts.peerClient.pairFunc = func(address string, port int, certFP, pin, clientName string) (string, string, error) {
		return "token-direct", "fp-direct", nil
	}

	if err := ts.svc.AddPeerByAddress("192.168.1.50", 47800, "111111"); err != nil {
		t.Fatalf("AddPeerByAddress() error = %v", err)
	}

	peer, err := ts.peers.Get("peer-2")
	if err != nil {
		t.Fatalf("peers.Get() error = %v", err)
	}
	if peer.Address != "192.168.1.50" || peer.Port != 47800 {
		t.Fatalf("stored peer = %+v", peer)
	}
}

func TestRemovePeer_DeletesPeerAndToken(t *testing.T) {
	ts := newTestService()
	if err := ts.peers.Save(domain.Peer{ID: "peer-1", PairedAt: time.Now()}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}
	if err := ts.secrets.Set(peerTokenRef("peer-1"), []byte("tok")); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	if err := ts.svc.RemovePeer("peer-1"); err != nil {
		t.Fatalf("RemovePeer() error = %v", err)
	}
	if ts.peers.count() != 0 {
		t.Fatalf("expected peer removed, count = %d", ts.peers.count())
	}
	if ts.secrets.has(peerTokenRef("peer-1")) {
		t.Fatal("expected token deleted")
	}
}

func TestFetchSharedHosts_CachesResultAndUpdatesLastSync(t *testing.T) {
	ts := newTestService()
	if err := ts.peers.Save(domain.Peer{ID: "peer-1", Address: "10.0.1.9", Port: 47800, PairedAt: time.Now()}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}
	if err := ts.secrets.Set(peerTokenRef("peer-1"), []byte("tok")); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}

	want := []domain.SharedHost{{Name: "web-prod-01", Address: "10.0.1.15", Port: 22, Username: "deploy"}}
	ts.peerClient.hostsFunc = func(address string, port int, certFP, token string) ([]domain.SharedHost, error) {
		if token != "tok" {
			t.Fatalf("FetchHosts() token = %q, want tok", token)
		}
		return want, nil
	}

	hosts, err := ts.svc.FetchSharedHosts("peer-1")
	if err != nil {
		t.Fatalf("FetchSharedHosts() error = %v", err)
	}
	if len(hosts) != 1 || hosts[0].Name != "web-prod-01" {
		t.Fatalf("hosts = %+v", hosts)
	}

	peer, err := ts.peers.Get("peer-1")
	if err != nil {
		t.Fatalf("peers.Get() error = %v", err)
	}
	if peer.LastSyncAt == nil {
		t.Fatal("expected LastSyncAt to be set")
	}
	if len(peer.Hosts) != 1 {
		t.Fatalf("expected cached hosts persisted, got %+v", peer.Hosts)
	}
}

func TestSyncLoop_KeepsCacheOnTransientFailure(t *testing.T) {
	withShrunkSyncInterval(t, 10*time.Millisecond)
	ts := newTestService()
	cached := []domain.SharedHost{{Name: "cached-host", Address: "10.0.1.15", Port: 22, Username: "deploy"}}
	if err := ts.peers.Save(domain.Peer{ID: "peer-1", Address: "10.0.1.9", Port: 47800, PairedAt: time.Now(), Hosts: cached}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}
	if err := ts.secrets.Set(peerTokenRef("peer-1"), []byte("tok")); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}
	ts.peerClient.hostsFunc = func(address string, port int, certFP, token string) ([]domain.SharedHost, error) {
		return nil, errors.New("network unreachable")
	}

	if err := ts.svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ts.svc.Close()

	time.Sleep(50 * time.Millisecond)

	peer, err := ts.peers.Get("peer-1")
	if err != nil {
		t.Fatalf("peers.Get() error = %v", err)
	}
	if len(peer.Hosts) != 1 || peer.Hosts[0].Name != "cached-host" {
		t.Fatalf("expected cache preserved on transient failure, got %+v", peer.Hosts)
	}
}

func TestImportSharedHost_SavesLocalHostWithSharedSource(t *testing.T) {
	ts := newTestService()
	if err := ts.peers.Save(domain.Peer{
		ID: "peer-1", Address: "10.0.1.9", Port: 47800, PairedAt: time.Now(),
		Hosts: []domain.SharedHost{{Name: "web-prod-01", Address: "10.0.1.15", Port: 22, Username: "deploy", Labels: []string{"prod"}}},
	}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}

	host, err := ts.svc.ImportSharedHost("peer-1", 0)
	if err != nil {
		t.Fatalf("ImportSharedHost() error = %v", err)
	}
	if host.ID == "" {
		t.Fatal("expected a generated ID")
	}
	if host.Name != "web-prod-01" || host.Address != "10.0.1.15" || host.Port != 22 || host.Username != "deploy" {
		t.Fatalf("host = %+v", host)
	}
	if host.Source != "shared:peer-1" {
		t.Fatalf("host.Source = %q, want shared:peer-1", host.Source)
	}

	stored, err := ts.hostRepo.Get(host.ID)
	if err != nil {
		t.Fatalf("hostRepo.Get() error = %v", err)
	}
	if stored.Source != "shared:peer-1" {
		t.Fatalf("stored host.Source = %q, want shared:peer-1", stored.Source)
	}
}

func TestImportSharedHost_IndexOutOfRangeReturnsError(t *testing.T) {
	ts := newTestService()
	if err := ts.peers.Save(domain.Peer{ID: "peer-1", PairedAt: time.Now(), Hosts: []domain.SharedHost{{Name: "only-host"}}}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}

	if _, err := ts.svc.ImportSharedHost("peer-1", 5); err == nil {
		t.Fatal("expected error for out-of-range index")
	}
	if _, err := ts.svc.ImportSharedHost("peer-1", -1); err == nil {
		t.Fatal("expected error for negative index")
	}
}

func TestImportSharedHost_UnknownPeerReturnsError(t *testing.T) {
	ts := newTestService()
	if _, err := ts.svc.ImportSharedHost("ghost", 0); err == nil {
		t.Fatal("expected error for unknown peer")
	}
}

func TestSyncLoop_DropsPeerOnUnauthorized(t *testing.T) {
	withShrunkSyncInterval(t, 10*time.Millisecond)
	ts := newTestService()
	if err := ts.peers.Save(domain.Peer{ID: "peer-1", Address: "10.0.1.9", Port: 47800, PairedAt: time.Now()}); err != nil {
		t.Fatalf("peers.Save() error = %v", err)
	}
	if err := ts.secrets.Set(peerTokenRef("peer-1"), []byte("revoked-tok")); err != nil {
		t.Fatalf("secrets.Set() error = %v", err)
	}
	ts.peerClient.hostsFunc = func(address string, port int, certFP, token string) ([]domain.SharedHost, error) {
		return nil, out.ErrPeerUnauthorized
	}

	if err := ts.svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ts.svc.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ts.peers.count() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expected peer to be dropped after unauthorized response")
}
