package share_test

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/adapter/in/sharehttp"
	"momo-shell/internal/adapter/out/shareclient"
	"momo-shell/internal/adapter/out/sqlite"
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
	"momo-shell/internal/core/service/share"
)

// This file proves the whole LAN-sharing chain end to end over real TLS on
// loopback: real sharehttp.Server (provider) <-> real shareclient.Client
// (consumer), real sqlite-backed repositories on both sides. It's the
// automated backbone of docs/plan/06-phase5-host-sharing.md §6's two-app
// manual acceptance walkthrough -- everything except real mDNS discovery
// (covered separately, and unreliable in sandboxed/CI environments) and
// pixels. Pairing goes through AddPeerByAddress (direct IP) rather than
// mDNS, matching the spec's own blocked-multicast fallback path.

// noopAnnouncer stands in for mDNS advertisement -- the provider instance
// in this test is never discovered, only dialed directly.
type noopAnnouncer struct{}

func (noopAnnouncer) Announce(instanceID, name string, port int) error { return nil }
func (noopAnnouncer) Stop()                                            {}

// capturingPublisher records the most recent share:pair-request payload so
// the test can extract its requestId (via JSON, since the payload type is
// unexported) and drive RespondPairing -- the same publish-then-block flow
// as the SSH host-key trust prompt.
type capturingPublisher struct {
	mu   sync.Mutex
	last any
}

func (p *capturingPublisher) Publish(topic string, payload any) {
	if topic != out.TopicSharePairRequest() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.last = payload
}

func (p *capturingPublisher) waitForRequestID(t *testing.T) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		payload := p.last
		p.last = nil
		p.mu.Unlock()
		if payload != nil {
			data, _ := json.Marshal(payload)
			var ev struct {
				RequestID string `json:"requestId"`
			}
			_ = json.Unmarshal(data, &ev)
			if ev.RequestID != "" {
				return ev.RequestID
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for share:pair-request event")
	return ""
}

// memSecretStore is a bare in-memory out.SecretStore, isolating this
// test's token storage from the real OS keychain/config dir.
type memSecretStore struct {
	mu      sync.Mutex
	secrets map[string][]byte
}

func newMemSecretStore() *memSecretStore { return &memSecretStore{secrets: map[string][]byte{}} }

func (s *memSecretStore) Set(ref string, v []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[ref] = v
	return nil
}

func (s *memSecretStore) Get(ref string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.secrets[ref]
	if !ok {
		return nil, errors.New("memSecretStore: not found")
	}
	return v, nil
}

func (s *memSecretStore) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, ref)
	return nil
}

func TestTwoInstancePairingAndSharing(t *testing.T) {
	// --- Provider ("B") ---
	dbB, err := sqlite.Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatalf("open db B: %v", err)
	}
	defer dbB.Close()

	hostRepoB := sqlite.NewHostRepo(dbB)
	if _, err := hostRepoB.Save(domain.Host{
		ID: "h1", Name: "web-prod-01", Address: "10.0.1.15", Port: 22,
		Username: "deploy", AuthType: domain.AuthPassword, Labels: []string{"prod"},
	}); err != nil {
		t.Fatalf("save host: %v", err)
	}

	pubB := &capturingPublisher{}
	serviceB := share.New(share.Deps{
		HostRepo:  hostRepoB,
		Clients:   sqlite.NewShareClientRepo(dbB),
		Settings:  sqlite.NewShareSettingsRepo(dbB),
		Pub:       pubB,
		Announcer: noopAnnouncer{},
	})

	certB, err := sharehttp.LoadOrCreateCert(t.TempDir())
	if err != nil {
		t.Fatalf("LoadOrCreateCert: %v", err)
	}
	serverB := sharehttp.New(serviceB, certB, sharehttp.WithListenAddrs([]string{"127.0.0.1:0"}))
	serviceB.SetServer(serverB)

	status, err := serviceB.EnableSharing([]string{"h1"})
	if err != nil {
		t.Fatalf("EnableSharing: %v", err)
	}

	// --- Consumer ("A") ---
	dbA, err := sqlite.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatalf("open db A: %v", err)
	}
	defer dbA.Close()

	secretsA := newMemSecretStore()
	serviceA := share.New(share.Deps{
		Peers:      sqlite.NewPeerRepo(dbA),
		Settings:   sqlite.NewShareSettingsRepo(dbA),
		Secrets:    secretsA,
		PeerClient: shareclient.New(),
		Pub:        &capturingPublisher{},
	})

	t.Run("deny", func(t *testing.T) {
		var wg sync.WaitGroup
		var pairErr error
		wg.Add(1)
		go func() {
			defer wg.Done()
			pairErr = serviceA.AddPeerByAddress("127.0.0.1", status.Port, status.PIN)
		}()

		requestID := pubB.waitForRequestID(t)
		if err := serviceB.RespondPairing(requestID, false); err != nil {
			t.Fatalf("RespondPairing() error = %v", err)
		}
		wg.Wait()

		if !errors.Is(pairErr, shareclient.ErrPairRejected) {
			t.Fatalf("AddPeerByAddress() error = %v, want ErrPairRejected", pairErr)
		}
	})

	var peerID, token string

	t.Run("approve and fetch hosts", func(t *testing.T) {
		var wg sync.WaitGroup
		var pairErr error
		wg.Add(1)
		go func() {
			defer wg.Done()
			pairErr = serviceA.AddPeerByAddress("127.0.0.1", status.Port, status.PIN)
		}()

		requestID := pubB.waitForRequestID(t)
		if err := serviceB.RespondPairing(requestID, true); err != nil {
			t.Fatalf("RespondPairing() error = %v", err)
		}
		wg.Wait()

		if pairErr != nil {
			t.Fatalf("AddPeerByAddress() error = %v", pairErr)
		}

		views, err := serviceA.ListPeers()
		if err != nil {
			t.Fatalf("ListPeers() error = %v", err)
		}
		if len(views) != 1 {
			t.Fatalf("ListPeers() = %+v, want 1 paired peer", views)
		}
		peerID = views[0].ID

		hosts, err := serviceA.FetchSharedHosts(peerID)
		if err != nil {
			t.Fatalf("FetchSharedHosts() error = %v", err)
		}
		if len(hosts) != 1 || hosts[0].Name != "web-prod-01" || hosts[0].Address != "10.0.1.15" || hosts[0].Username != "deploy" {
			t.Fatalf("FetchSharedHosts() = %+v", hosts)
		}

		clients, err := serviceB.ListClients()
		if err != nil || len(clients) != 1 {
			t.Fatalf("ListClients() = %+v, %v, want 1 client", clients, err)
		}

		// sharetoken:{peerID} mirrors consumer.go's peerTokenRef convention
		// -- unexported, so duplicated here rather than imported.
		tokenBytes, err := secretsA.Get("sharetoken:" + peerID)
		if err != nil {
			t.Fatalf("secretsA.Get() error = %v", err)
		}
		token = string(tokenBytes)

		// Acceptance criterion 3, at the wire level: dump the raw HTTPS
		// response and assert no credential material is present anywhere
		// in the payload, not just in the Go struct shape (already covered
		// by provider_test.go's TestSharedHost_JSONHasExactlyFiveKeys...).
		dumpAndVerifyNoCredentials(t, status.Port, token)
	})

	t.Run("revoke drops access on next fetch", func(t *testing.T) {
		clients, err := serviceB.ListClients()
		if err != nil || len(clients) != 1 {
			t.Fatalf("ListClients() = %+v, %v, want 1 client", clients, err)
		}
		if err := serviceB.RevokeClient(clients[0].ID); err != nil {
			t.Fatalf("RevokeClient() error = %v", err)
		}

		if _, err := serviceA.FetchSharedHosts(peerID); !errors.Is(err, out.ErrPeerUnauthorized) {
			t.Fatalf("FetchSharedHosts() (after revoke) error = %v, want ErrPeerUnauthorized", err)
		}
	})

	// Run last: this permanently locks out serviceB's PIN for 60s, so no
	// further pairing against B happens afterward in this test.
	t.Run("wrong pin five times locks out", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			err := serviceA.AddPeerByAddress("127.0.0.1", status.Port, "000000")
			if !errors.Is(err, shareclient.ErrPairRejected) {
				t.Fatalf("attempt %d: err = %v, want ErrPairRejected", i, err)
			}
		}
		if err := serviceA.AddPeerByAddress("127.0.0.1", status.Port, "000000"); !errors.Is(err, shareclient.ErrPairLockedOut) {
			t.Fatalf("after 5 failures: err = %v, want ErrPairLockedOut", err)
		}
	})
}

// dumpAndVerifyNoCredentials performs a raw HTTPS request against
// /api/v1/hosts (bypassing shareclient/Service entirely) and asserts the
// response body -- byte for byte what a packet capture would show -- never
// contains credential material, satisfying acceptance criterion 3 at the
// actual wire level rather than just the Go struct shape.
func dumpAndVerifyNoCredentials(t *testing.T, port int, token string) {
	t.Helper()

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://127.0.0.1:%d/api/v1/hosts", port), nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /hosts error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /hosts status = %d, want 200", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode /hosts response: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("raw /hosts response = %+v, want 1 entry", raw)
	}

	wantKeys := map[string]bool{"name": true, "address": true, "port": true, "labels": true, "username": true}
	for k := range raw[0] {
		if !wantKeys[k] {
			t.Fatalf("unexpected key %q in wire response: %+v", k, raw[0])
		}
	}
	for _, forbidden := range []string{"id", "authType", "keyPath", "password", "secret", "passphrase"} {
		if _, ok := raw[0][forbidden]; ok {
			t.Fatalf("wire response must never contain %q: %+v", forbidden, raw[0])
		}
	}
}
