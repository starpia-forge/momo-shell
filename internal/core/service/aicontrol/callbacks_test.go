package aicontrol

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// withShrunkPairApprovalTimeout shrinks pairApprovalTimeout for the duration
// of a test so timeout tests don't wait the full 60s (connect_test.go's
// withShrunkApprovalTimeout mirror).
func withShrunkPairApprovalTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	original := pairApprovalTimeout
	pairApprovalTimeout = d
	t.Cleanup(func() { pairApprovalTimeout = original })
}

func waitForPairRequest(t *testing.T, pub *recordingPublisher) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range pub.all() {
			if payload, ok := e.payload.(mcpPairRequestPayload); ok {
				return payload.RequestID
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for mcp:pair-request event")
	return ""
}

// newCallbacksTestService builds a Service wired with a fakeMCPClients, for
// pairing/auth tests. Kept separate from newTestService (connect_test.go)
// so that helper's signature -- used by every connect/control/run test --
// stays unchanged.
func newCallbacksTestService() (*Service, *recordingPublisher, *fakeMCPClients) {
	pub := &recordingPublisher{}
	clients := newFakeMCPClients()
	svc := New(Deps{Publisher: pub, Clients: clients})
	return svc, pub, clients
}

func TestHandlePair_ApproveIssuesToken(t *testing.T) {
	svc, pub, clients := newCallbacksTestService()

	var wg sync.WaitGroup
	var token string
	var pairErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		token, pairErr = svc.HandlePair(context.Background(), "claude-desktop")
	}()

	requestID := waitForPairRequest(t, pub)
	if err := svc.RespondPairing(requestID, true); err != nil {
		t.Fatalf("RespondPairing() error = %v", err)
	}
	wg.Wait()

	if pairErr != nil {
		t.Fatalf("HandlePair() error = %v", pairErr)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if clients.count() != 1 {
		t.Fatalf("clients.count() = %d, want 1", clients.count())
	}
}

func TestHandlePair_DenyReturnsErrPairDenied(t *testing.T) {
	svc, pub, _ := newCallbacksTestService()

	var wg sync.WaitGroup
	var pairErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, pairErr = svc.HandlePair(context.Background(), "peer")
	}()

	requestID := waitForPairRequest(t, pub)
	if err := svc.RespondPairing(requestID, false); err != nil {
		t.Fatalf("RespondPairing() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(pairErr, in.ErrPairDenied) {
		t.Fatalf("HandlePair() error = %v, want ErrPairDenied", pairErr)
	}
}

func TestHandlePair_TimeoutReturnsErrPairTimeout(t *testing.T) {
	withShrunkPairApprovalTimeout(t, 20*time.Millisecond)
	svc, _, _ := newCallbacksTestService()

	_, pairErr := svc.HandlePair(context.Background(), "peer")
	if !errors.Is(pairErr, in.ErrPairTimeout) {
		t.Fatalf("HandlePair() error = %v, want ErrPairTimeout", pairErr)
	}
}

// TestHandlePair_ContextCanceledDuringApprovalSkipsTokenIssuance is this
// cycle's key contract test (user-confirmed disconnect guard, A2): if the
// mcpipc adapter's ctx is canceled -- the connecting client disconnected --
// while HandlePair is still waiting for the local user's decision, a
// RespondPairing(true) that arrives afterward must not mint and persist a
// ghost client no one is there to receive (share HS-03 mirror).
func TestHandlePair_ContextCanceledDuringApprovalSkipsTokenIssuance(t *testing.T) {
	svc, pub, clients := newCallbacksTestService()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var pairErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, pairErr = svc.HandlePair(ctx, "peer")
	}()

	requestID := waitForPairRequest(t, pub)
	cancel()
	wg.Wait()

	if !errors.Is(pairErr, in.ErrPairTimeout) {
		t.Fatalf("HandlePair() error = %v, want ErrPairTimeout", pairErr)
	}

	if err := svc.RespondPairing(requestID, true); err == nil {
		t.Fatal("RespondPairing() after cancellation: expected error (no pending request), got nil")
	}
	if clients.count() != 0 {
		t.Fatalf("clients.count() = %d, want 0 (no ghost client)", clients.count())
	}
}

func TestRespondPairing_UnknownRequestIDReturnsError(t *testing.T) {
	svc, _, _ := newCallbacksTestService()
	if err := svc.RespondPairing("bogus", true); !errors.Is(err, ErrNoPendingPairApproval) {
		t.Fatalf("RespondPairing() error = %v, want ErrNoPendingPairApproval", err)
	}
}

func TestRespondPairing_AlreadyAnswered(t *testing.T) {
	svc, pub, _ := newCallbacksTestService()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.HandlePair(context.Background(), "peer")
	}()

	requestID := waitForPairRequest(t, pub)
	if err := svc.RespondPairing(requestID, true); err != nil {
		t.Fatalf("first RespondPairing() error = %v", err)
	}
	wg.Wait()

	if err := svc.RespondPairing(requestID, true); !errors.Is(err, ErrNoPendingPairApproval) {
		t.Fatalf("RespondPairing() after resolution: error = %v, want ErrNoPendingPairApproval (already cleaned up)", err)
	}
}

func TestAuthClient_ValidToken(t *testing.T) {
	svc, pub, clients := newCallbacksTestService()

	var wg sync.WaitGroup
	var token string
	wg.Add(1)
	go func() {
		defer wg.Done()
		token, _ = svc.HandlePair(context.Background(), "peer")
	}()
	requestID := waitForPairRequest(t, pub)
	_ = svc.RespondPairing(requestID, true)
	wg.Wait()

	all, err := clients.List()
	if err != nil || len(all) != 1 {
		t.Fatalf("clients.List() = %v, %v, want 1 client", all, err)
	}
	wantClientID := all[0].ClientID

	clientID, err := svc.AuthClient(token)
	if err != nil {
		t.Fatalf("AuthClient() error = %v", err)
	}
	if clientID != wantClientID {
		t.Fatalf("AuthClient() clientID = %q, want %q", clientID, wantClientID)
	}
	if !clients.wasTouched(wantClientID) {
		t.Error("expected TouchSeen to be called for the authenticated client")
	}
}

func TestAuthClient_UnknownTokenReturnsErrUnauthorized(t *testing.T) {
	svc, _, _ := newCallbacksTestService()
	if _, err := svc.AuthClient("bogus-token"); !errors.Is(err, in.ErrUnauthorized) {
		t.Fatalf("AuthClient() error = %v, want ErrUnauthorized", err)
	}
}

func TestAuthClient_RevokedTokenReturnsErrUnauthorized(t *testing.T) {
	svc, _, clients := newCallbacksTestService()
	clients.seed(domain.MCPClient{ClientID: "client-1", Name: "peer", Revoked: true}, hashToken("revoked-token"))

	if _, err := svc.AuthClient("revoked-token"); !errors.Is(err, in.ErrUnauthorized) {
		t.Fatalf("AuthClient() error = %v, want ErrUnauthorized", err)
	}
}
