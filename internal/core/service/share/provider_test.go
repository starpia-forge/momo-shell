package share

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// withShrunkApprovalTimeout shrinks pairApprovalTimeout for the duration of
// a test so timeout tests don't wait the full 60s.
func withShrunkApprovalTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	original := pairApprovalTimeout
	pairApprovalTimeout = d
	t.Cleanup(func() { pairApprovalTimeout = original })
}

func TestHandlePair_WrongPinFiveTimesLocksOut(t *testing.T) {
	ts := newTestService()
	if _, err := ts.svc.EnableSharing(nil); err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	for i := 0; i < maxPinFailures; i++ {
		if _, err := ts.svc.HandlePair(context.Background(), "000000", "peer", "10.0.0.5:1234"); !errors.Is(err, in.ErrPinMismatch) {
			t.Fatalf("attempt %d: err = %v, want ErrPinMismatch", i, err)
		}
	}

	if _, err := ts.svc.HandlePair(context.Background(), "000000", "peer", "10.0.0.5:1234"); !errors.Is(err, in.ErrLockedOut) {
		t.Fatalf("after %d failures: err = %v, want ErrLockedOut", maxPinFailures, err)
	}
}

func TestHandlePair_ApproveIssuesToken(t *testing.T) {
	ts := newTestService()
	status, err := ts.svc.EnableSharing([]string{"h1"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	var wg sync.WaitGroup
	var token string
	var pairErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		token, pairErr = ts.svc.HandlePair(context.Background(), status.PIN, "kim-laptop", "10.0.0.5:1234")
	}()

	requestID := waitForPairRequest(t, ts.pub)
	if err := ts.svc.RespondPairing(requestID, true); err != nil {
		t.Fatalf("RespondPairing() error = %v", err)
	}
	wg.Wait()

	if pairErr != nil {
		t.Fatalf("HandlePair() error = %v", pairErr)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if ts.clients.count() != 1 {
		t.Fatalf("clients.count() = %d, want 1", ts.clients.count())
	}
}

func TestHandlePair_DenyReturnsErrPairDenied(t *testing.T) {
	ts := newTestService()
	status, err := ts.svc.EnableSharing(nil)
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	var wg sync.WaitGroup
	var pairErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, pairErr = ts.svc.HandlePair(context.Background(), status.PIN, "peer", "10.0.0.5:1234")
	}()

	requestID := waitForPairRequest(t, ts.pub)
	if err := ts.svc.RespondPairing(requestID, false); err != nil {
		t.Fatalf("RespondPairing() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(pairErr, in.ErrPairDenied) {
		t.Fatalf("HandlePair() error = %v, want ErrPairDenied", pairErr)
	}
}

func TestHandlePair_TimeoutReturnsErrPairTimeout(t *testing.T) {
	withShrunkApprovalTimeout(t, 20*time.Millisecond)
	ts := newTestService()
	status, err := ts.svc.EnableSharing(nil)
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	_, pairErr := ts.svc.HandlePair(context.Background(), status.PIN, "peer", "10.0.0.5:1234")
	if !errors.Is(pairErr, in.ErrPairTimeout) {
		t.Fatalf("HandlePair() error = %v, want ErrPairTimeout", pairErr)
	}
}

// TestHandlePair_ContextCanceledDuringApprovalSkipsTokenIssuance guards
// against the HS-03 ghost-client bug: if the caller (the HTTP request)
// disconnects while HandlePair is still waiting for the local user's
// approve/deny decision, a RespondPairing(true) that arrives afterward must
// not mint and persist a client no one is there to receive.
func TestHandlePair_ContextCanceledDuringApprovalSkipsTokenIssuance(t *testing.T) {
	ts := newTestService()
	status, err := ts.svc.EnableSharing(nil)
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var pairErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, pairErr = ts.svc.HandlePair(ctx, status.PIN, "peer", "10.0.0.5:1234")
	}()

	requestID := waitForPairRequest(t, ts.pub)
	cancel()
	wg.Wait()

	if !errors.Is(pairErr, in.ErrPairTimeout) {
		t.Fatalf("HandlePair() error = %v, want ErrPairTimeout", pairErr)
	}

	// A decision arriving after the caller gave up must be a no-op --
	// the pending request was already cleaned up by awaitPairApproval.
	if err := ts.svc.RespondPairing(requestID, true); err == nil {
		t.Fatal("RespondPairing() after cancellation: expected error (no pending request), got nil")
	}
	if ts.clients.count() != 0 {
		t.Fatalf("clients.count() = %d, want 0 (no ghost client)", ts.clients.count())
	}
}

func TestHandlePair_WhileDisabledReturnsUnauthorized(t *testing.T) {
	ts := newTestService()
	if _, err := ts.svc.HandlePair(context.Background(), "123456", "peer", "10.0.0.5:1234"); !errors.Is(err, in.ErrShareUnauthorized) {
		t.Fatalf("err = %v, want ErrShareUnauthorized", err)
	}
}

func TestHostsForToken_ValidTokenReturnsSharedHosts(t *testing.T) {
	ts := newTestService()
	ts.hostRepo.hosts["h1"] = domain.Host{ID: "h1", Name: "web-prod-01", Address: "10.0.1.15", Port: 22, Username: "deploy", Labels: []string{"prod"}}
	ts.hostRepo.hosts["h2"] = domain.Host{ID: "h2", Name: "db-staging", Address: "10.0.1.20", Port: 22, Username: "deploy"}

	status, err := ts.svc.EnableSharing([]string{"h1", "h2"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	var wg sync.WaitGroup
	var token string
	wg.Add(1)
	go func() {
		defer wg.Done()
		token, _ = ts.svc.HandlePair(context.Background(), status.PIN, "kim-laptop", "10.0.0.5:1234")
	}()
	requestID := waitForPairRequest(t, ts.pub)
	_ = ts.svc.RespondPairing(requestID, true)
	wg.Wait()

	hosts, err := ts.svc.HostsForToken(token)
	if err != nil {
		t.Fatalf("HostsForToken() error = %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("HostsForToken() = %+v, want 2 hosts", hosts)
	}
	if ts.clients.count() != 1 {
		t.Fatalf("clients.count() = %d, want 1", ts.clients.count())
	}
}

func TestHostsForToken_InvalidTokenReturnsUnauthorized(t *testing.T) {
	ts := newTestService()
	if _, err := ts.svc.EnableSharing(nil); err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}
	if _, err := ts.svc.HostsForToken("not-a-real-token"); !errors.Is(err, in.ErrShareUnauthorized) {
		t.Fatalf("err = %v, want ErrShareUnauthorized", err)
	}
}

func TestHostsForToken_SkipsHostsDeletedSinceSelection(t *testing.T) {
	ts := newTestService()
	ts.hostRepo.hosts["h1"] = domain.Host{ID: "h1", Name: "still-here", Address: "10.0.1.15", Port: 22, Username: "deploy"}
	// h2 intentionally never added to hostRepo -- simulates deletion after selection.

	status, err := ts.svc.EnableSharing([]string{"h1", "h2"})
	if err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	var wg sync.WaitGroup
	var token string
	wg.Add(1)
	go func() {
		defer wg.Done()
		token, _ = ts.svc.HandlePair(context.Background(), status.PIN, "peer", "10.0.0.5:1234")
	}()
	requestID := waitForPairRequest(t, ts.pub)
	_ = ts.svc.RespondPairing(requestID, true)
	wg.Wait()

	hosts, err := ts.svc.HostsForToken(token)
	if err != nil {
		t.Fatalf("HostsForToken() error = %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("HostsForToken() = %+v, want 1 host (deleted one skipped)", hosts)
	}
}

// TestSharedHost_JSONHasExactlyFiveKeysAndNoCredentialMaterial structurally
// enforces the spec's core security invariant (acceptance criterion 3):
// SharedHost's wire shape can never carry secret or structural fields.
func TestSharedHost_JSONHasExactlyFiveKeysAndNoCredentialMaterial(t *testing.T) {
	h := domain.SharedHost{Name: "web-prod-01", Address: "10.0.1.15", Port: 22, Labels: []string{"prod"}, Username: "deploy"}

	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var asMap map[string]any
	if err := json.Unmarshal(data, &asMap); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	wantKeys := map[string]bool{"name": true, "address": true, "port": true, "labels": true, "username": true}
	if len(asMap) != len(wantKeys) {
		t.Fatalf("SharedHost JSON has %d keys, want %d: %v", len(asMap), len(wantKeys), asMap)
	}
	for k := range asMap {
		if !wantKeys[k] {
			t.Fatalf("unexpected key %q in SharedHost JSON: %v", k, asMap)
		}
	}
	for _, forbidden := range []string{"id", "authType", "keyPath", "password", "secret", "passphrase"} {
		if _, ok := asMap[forbidden]; ok {
			t.Fatalf("SharedHost JSON must never contain %q, got: %v", forbidden, asMap)
		}
	}
}

func TestInfo_ReflectsSharedHostCount(t *testing.T) {
	ts := newTestService()
	if _, err := ts.svc.EnableSharing([]string{"h1", "h2", "h3"}); err != nil {
		t.Fatalf("EnableSharing() error = %v", err)
	}

	info := ts.svc.Info()
	if info.HostCount != 3 {
		t.Fatalf("Info().HostCount = %d, want 3", info.HostCount)
	}
	if info.ID == "" {
		t.Fatal("expected non-empty instance ID")
	}
}

// waitForPairRequest polls the recording publisher for a share:pair-request
// event and returns its requestId, failing the test if none appears.
func waitForPairRequest(t *testing.T, pub *recordingPublisher) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range pub.all() {
			if payload, ok := e.payload.(pairRequestPayload); ok {
				return payload.RequestID
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for share:pair-request event")
	return ""
}
