package aicontrol

import (
	"errors"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func waitForConnectionScopeApprovalRequest(t *testing.T, pub *recordingPublisher) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range pub.all() {
			if payload, ok := e.payload.(connectionScopePayload); ok {
				return payload.RequestID
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for mcp:connection-scope-approval event")
	return ""
}

func TestRequestConnectionScope_ApproveGrantsScope(t *testing.T) {
	web1 := domain.Host{ID: "host-1", Name: "web-1"}
	web2 := domain.Host{ID: "host-2", Name: "web-2"}
	svc, pub, _ := newTestService(web1, web2)

	var wg sync.WaitGroup
	var scope domain.ConnectionScope
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		scope, reqErr = svc.RequestConnectionScope("client-1", []string{"web-1", "web-2"})
	}()

	requestID := waitForConnectionScopeApprovalRequest(t, pub)
	if err := svc.RespondConnectionScopeApproval(requestID, true); err != nil {
		t.Fatalf("RespondConnectionScopeApproval() error = %v", err)
	}
	wg.Wait()

	if reqErr != nil {
		t.Fatalf("RequestConnectionScope() error = %v", reqErr)
	}
	if scope.ClientID != "client-1" {
		t.Errorf("scope.ClientID = %q, want client-1", scope.ClientID)
	}
	wantHosts := map[string]bool{"web-1": true, "web-2": true}
	if len(scope.HostNames) != len(wantHosts) || !scope.HostNames["web-1"] || !scope.HostNames["web-2"] {
		t.Errorf("scope.HostNames = %+v, want %+v", scope.HostNames, wantHosts)
	}
	if scope.MaxConcurrent != 2 {
		t.Errorf("scope.MaxConcurrent = %d, want 2", scope.MaxConcurrent)
	}
}

func TestRequestConnectionScope_DenyReturnsErr(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, _ := newTestService(host)

	var wg sync.WaitGroup
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, reqErr = svc.RequestConnectionScope("client-1", []string{"web-1"})
	}()

	requestID := waitForConnectionScopeApprovalRequest(t, pub)
	if err := svc.RespondConnectionScopeApproval(requestID, false); err != nil {
		t.Fatalf("RespondConnectionScopeApproval() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(reqErr, ErrConnectionScopeDenied) {
		t.Fatalf("RequestConnectionScope() error = %v, want ErrConnectionScopeDenied", reqErr)
	}
}

func TestRequestConnectionScope_UnknownHostRejectedWithoutPrompt(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, _ := newTestService(host)

	_, err := svc.RequestConnectionScope("client-1", []string{"web-1", "does-not-exist"})

	if !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("RequestConnectionScope() error = %v, want ErrHostNotFound", err)
	}
	if len(pub.all()) != 0 {
		t.Error("expected no approval event to be published for a request naming an unknown host")
	}
}

func TestRequestConnectionScope_EmptyHostNamesRejected(t *testing.T) {
	svc, pub, _ := newTestService()

	_, err := svc.RequestConnectionScope("client-1", nil)

	if !errors.Is(err, ErrEmptyHostNames) {
		t.Fatalf("RequestConnectionScope() error = %v, want ErrEmptyHostNames", err)
	}
	if len(pub.all()) != 0 {
		t.Error("expected no approval event to be published for an empty host list")
	}
}

// grantScope is a test helper that drives RequestConnectionScope to
// completion with an approval, returning the granted scope.
func grantScope(t *testing.T, svc *Service, pub *recordingPublisher, clientID string, hostNames []string) domain.ConnectionScope {
	t.Helper()
	var wg sync.WaitGroup
	var scope domain.ConnectionScope
	wg.Add(1)
	go func() {
		defer wg.Done()
		scope, _ = svc.RequestConnectionScope(clientID, hostNames)
	}()
	requestID := waitForConnectionScopeApprovalRequest(t, pub)
	if err := svc.RespondConnectionScopeApproval(requestID, true); err != nil {
		t.Fatalf("RespondConnectionScopeApproval() error = %v", err)
	}
	wg.Wait()
	return scope
}

func TestConnectHost_WithinScope_AutoApprovesWithoutPrompt(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, _ := newTestService(host)
	scope := grantScope(t, svc, pub, "client-1", []string{"web-1"})

	view, err := svc.ConnectHost("client-1", "web-1")

	if err != nil {
		t.Fatalf("ConnectHost() error = %v", err)
	}
	for _, e := range pub.all() {
		if _, ok := e.payload.(connectApprovalPayload); ok {
			t.Error("expected no mcp:connect-approval prompt for a connect within an active scope")
		}
	}
	// The auto-grant itself still publishes mcp:delegation (B5a) -- the
	// fan-out path has no approval dialog to piggyback on, so this is the
	// only signal the frontend gets for an auto-approved session.
	payload := lastDelegationPayload(t, pub)
	if payload.SessionID != view.ID || payload.State != "delegated" || payload.Reason != "granted" {
		t.Errorf("payload = %+v, want SessionID=%s State=delegated Reason=granted", payload, view.ID)
	}

	svc.mu.Lock()
	deleg, ok := svc.delegations[view.ID]
	svc.mu.Unlock()
	if !ok {
		t.Fatal("expected a delegation to be recorded")
	}
	if deleg.ScopeID != scope.ID {
		t.Errorf("delegation.ScopeID = %q, want %q", deleg.ScopeID, scope.ID)
	}
}

func TestConnectHost_OutsideScope_StillRequiresApproval(t *testing.T) {
	web1 := domain.Host{ID: "host-1", Name: "web-1"}
	web2 := domain.Host{ID: "host-2", Name: "web-2"}
	svc, pub, _ := newTestService(web1, web2)
	grantScope(t, svc, pub, "client-1", []string{"web-1"}) // scope covers web-1 only

	var wg sync.WaitGroup
	var view domain.SessionView
	var connectErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		view, connectErr = svc.ConnectHost("client-1", "web-2") // outside the scope
	}()

	requestID := waitForConnectApprovalRequest(t, pub)
	if err := svc.RespondConnectApproval(requestID, true); err != nil {
		t.Fatalf("RespondConnectApproval() error = %v", err)
	}
	wg.Wait()

	if connectErr != nil {
		t.Fatalf("ConnectHost() error = %v", connectErr)
	}
	svc.mu.Lock()
	deleg := svc.delegations[view.ID]
	svc.mu.Unlock()
	if deleg == nil || deleg.ScopeID != "" {
		t.Errorf("delegation.ScopeID = %+v, want empty -- web-2 is outside the granted scope", deleg)
	}
}

func TestConnectHost_ScopeCapReached_FallsBackToApproval(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, sessions := newTestService(host)
	grantScope(t, svc, pub, "client-1", []string{"web-1"}) // MaxConcurrent == 1

	// First connect: auto-approved (0 < 1), fills the cap.
	if _, err := svc.ConnectHost("client-1", "web-1"); err != nil {
		t.Fatalf("first ConnectHost() error = %v", err)
	}

	// Make the fake session layer report that session as still alive, so
	// the cap check on the next attempt sees it as live.
	sessions.snapshotFunc = func() []domain.Session {
		return []domain.Session{{ID: "sess-1"}}
	}

	// Second connect to the same (only) scoped host: cap is now reached, so
	// it must fall back to the individual approval path instead of
	// auto-granting a second time.
	var wg sync.WaitGroup
	var connectErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, connectErr = svc.ConnectHost("client-1", "web-1")
	}()

	requestID := waitForConnectApprovalRequest(t, pub)
	if err := svc.RespondConnectApproval(requestID, false); err != nil {
		t.Fatalf("RespondConnectApproval() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(connectErr, ErrConnectDenied) {
		t.Fatalf("second ConnectHost() error = %v, want ErrConnectDenied (proves it fell back to individual approval)", connectErr)
	}
}

func TestLiveDelegationCountForScope_ReapsDeadSessions(t *testing.T) {
	svc, _, sessions := newTestService()
	sessions.snapshotFunc = func() []domain.Session { return nil } // nothing is live

	svc.mu.Lock()
	svc.delegations["sess-dead"] = &domain.Delegation{SessionID: "sess-dead", ClientID: "client-1", ScopeID: "scope-1"}
	svc.mu.Unlock()

	count := svc.liveDelegationCountForScope("scope-1")

	if count != 0 {
		t.Errorf("liveDelegationCountForScope() = %d, want 0", count)
	}
	svc.mu.Lock()
	_, stillThere := svc.delegations["sess-dead"]
	svc.mu.Unlock()
	if stillThere {
		t.Error("expected the dead session's delegation to be reaped")
	}
}
