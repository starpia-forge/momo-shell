package mdns

import (
	"os"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/port/out"
)

// TestAnnounceAndBrowse_Loopback is a real-network smoke test. Multicast
// mDNS is commonly blocked or unreliable in sandboxed/CI environments
// (confirmed in this project's own dev sandbox), so it's opt-in via
// MOMO_TEST_MDNS rather than run by default -- `go test ./...` must stay
// deterministic. The core pairing/consumer logic is covered by
// core/service/share's fake-driven unit tests; this only checks the wire
// format (TXT id=/name=) round-trips through a real Register/Browse pair.
// Run manually with: MOMO_TEST_MDNS=1 go test ./internal/adapter/out/mdns/...
func TestAnnounceAndBrowse_Loopback(t *testing.T) {
	if os.Getenv("MOMO_TEST_MDNS") == "" {
		t.Skip("skipping real mDNS test (set MOMO_TEST_MDNS=1 to run)")
	}

	announcer := NewAnnouncer()
	if err := announcer.Announce("test-instance-1", "test-host", 47800); err != nil {
		t.Fatalf("Announce() error = %v", err)
	}
	defer announcer.Stop()

	browser := NewBrowser()
	var (
		mu    sync.Mutex
		found bool
	)
	err := browser.Start(func(peers []out.DiscoveredPeer) {
		mu.Lock()
		defer mu.Unlock()
		for _, p := range peers {
			if p.InstanceID == "test-instance-1" && p.Name == "test-host" {
				found = true
			}
		}
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer browser.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := found
		mu.Unlock()
		if ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("timed out waiting to discover announced instance over mDNS")
}
