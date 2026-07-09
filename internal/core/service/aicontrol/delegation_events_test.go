package aicontrol

import (
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func TestConnectHost_PublishesDelegationGranted(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, _ := newTestService(host)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.ConnectHost("client-1", "web-1")
	}()
	requestID := waitForConnectApprovalRequest(t, pub)
	if err := svc.RespondConnectApproval(requestID, true); err != nil {
		t.Fatalf("RespondConnectApproval() error = %v", err)
	}
	wg.Wait()

	payload := lastDelegationPayload(t, pub)
	if payload.SessionID != "sess-1" || payload.ClientID != "client-1" {
		t.Errorf("payload session/client = %q/%q, want sess-1/client-1", payload.SessionID, payload.ClientID)
	}
	if payload.State != "delegated" || payload.Reason != "granted" {
		t.Errorf("payload state/reason = %q/%q, want delegated/granted", payload.State, payload.Reason)
	}
	if !payload.AICreated {
		t.Error("expected AICreated=true for a ConnectHost-opened session")
	}
}

func TestRequestControl_PublishesDelegationGranted(t *testing.T) {
	svc, pub, sessions := newTestService()
	sessions.sessionShellFunc = existingSession("sess-1")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.RequestControl("client-1", "sess-1", domain.ControlScope{})
	}()
	requestID := waitForControlApprovalRequest(t, pub)
	if err := svc.RespondControlApproval(requestID, true); err != nil {
		t.Fatalf("RespondControlApproval() error = %v", err)
	}
	wg.Wait()

	payload := lastDelegationPayload(t, pub)
	if payload.State != "delegated" || payload.Reason != "granted" {
		t.Errorf("payload state/reason = %q/%q, want delegated/granted", payload.State, payload.Reason)
	}
	if payload.AICreated {
		t.Error("expected AICreated=false for a borrowed user session")
	}
}

func TestReleaseControl_PublishesDelegationReleased(t *testing.T) {
	svc, pub, _ := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if err := svc.ReleaseControl("client-1", "sess-1"); err != nil {
		t.Fatalf("ReleaseControl() error = %v", err)
	}

	payload := lastDelegationPayload(t, pub)
	if payload.State != "none" || payload.Reason != "released" {
		t.Errorf("payload state/reason = %q/%q, want none/released", payload.State, payload.Reason)
	}
}

func TestKillControl_PublishesDelegationKilled(t *testing.T) {
	svc, pub, _ := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if err := svc.KillControl("sess-1"); err != nil {
		t.Fatalf("KillControl() error = %v", err)
	}

	payload := lastDelegationPayload(t, pub)
	if payload.State != "none" || payload.Reason != "killed" {
		t.Errorf("payload state/reason = %q/%q, want none/killed", payload.State, payload.Reason)
	}
	if payload.ClientID != "client-1" {
		t.Errorf("payload.ClientID = %q, want client-1", payload.ClientID)
	}
}

func TestListDelegations_ReturnsActiveOnly(t *testing.T) {
	svc, _, _ := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	seedDelegation(svc, "sess-2", "client-2", time.Now())

	list, err := svc.ListDelegations()
	if err != nil {
		t.Fatalf("ListDelegations() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListDelegations() len = %d, want 2", len(list))
	}

	if err := svc.ReleaseControl("client-1", "sess-1"); err != nil {
		t.Fatalf("ReleaseControl() error = %v", err)
	}
	list, err = svc.ListDelegations()
	if err != nil {
		t.Fatalf("ListDelegations() error = %v", err)
	}
	if len(list) != 1 || list[0].SessionID != "sess-2" {
		t.Fatalf("ListDelegations() = %+v, want only sess-2", list)
	}
}

// lastDelegationPayload returns the most recently published
// delegationPayload, failing the test if none was published.
func lastDelegationPayload(t *testing.T, pub *recordingPublisher) delegationPayload {
	t.Helper()
	events := pub.all()
	for i := len(events) - 1; i >= 0; i-- {
		if payload, ok := events[i].payload.(delegationPayload); ok {
			return payload
		}
	}
	t.Fatal("expected an mcp:delegation event to be published")
	return delegationPayload{}
}
