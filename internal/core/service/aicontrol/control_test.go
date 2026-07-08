package aicontrol

import (
	"errors"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func withShrunkControlApprovalTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	original := controlApprovalTimeout
	controlApprovalTimeout = d
	t.Cleanup(func() { controlApprovalTimeout = original })
}

func waitForControlApprovalRequest(t *testing.T, pub *recordingPublisher) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range pub.all() {
			if payload, ok := e.payload.(controlApprovalPayload); ok {
				return payload.RequestID
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for mcp:control-approval event")
	return ""
}

func existingSession(sessionID string) func(id string) (string, domain.SessionKind, bool) {
	return func(id string) (string, domain.SessionKind, bool) {
		if id == sessionID {
			return "bash", domain.KindLocal, true
		}
		return "", "", false
	}
}

func TestRequestControl_ApproveGrantsDelegation(t *testing.T) {
	svc, pub, sessions := newTestService()
	sessions.sessionShellFunc = existingSession("sess-1")

	var wg sync.WaitGroup
	var deleg domain.Delegation
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		deleg, reqErr = svc.RequestControl("client-1", "sess-1", domain.ControlScope{HostOnly: true})
	}()

	requestID := waitForControlApprovalRequest(t, pub)
	if err := svc.RespondControlApproval(requestID, true); err != nil {
		t.Fatalf("RespondControlApproval() error = %v", err)
	}
	wg.Wait()

	if reqErr != nil {
		t.Fatalf("RequestControl() error = %v", reqErr)
	}
	if deleg.State != domain.DelegActive {
		t.Errorf("State = %v, want %v", deleg.State, domain.DelegActive)
	}
	if deleg.SessionID != "sess-1" || deleg.ClientID != "client-1" {
		t.Errorf("SessionID/ClientID = %q/%q, want sess-1/client-1", deleg.SessionID, deleg.ClientID)
	}
	if deleg.Scope != (domain.ControlScope{HostOnly: true}) {
		t.Errorf("Scope = %+v, want HostOnly=true", deleg.Scope)
	}
}

func TestRequestControl_DenyReturnsErrControlDenied(t *testing.T) {
	svc, pub, sessions := newTestService()
	sessions.sessionShellFunc = existingSession("sess-1")

	var wg sync.WaitGroup
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, reqErr = svc.RequestControl("client-1", "sess-1", domain.ControlScope{})
	}()

	requestID := waitForControlApprovalRequest(t, pub)
	if err := svc.RespondControlApproval(requestID, false); err != nil {
		t.Fatalf("RespondControlApproval() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(reqErr, ErrControlDenied) {
		t.Fatalf("RequestControl() error = %v, want ErrControlDenied", reqErr)
	}
}

func TestRequestControl_UnknownSessionIsRejected(t *testing.T) {
	svc, _, sessions := newTestService()
	sessions.sessionShellFunc = existingSession("some-other-session")

	_, err := svc.RequestControl("client-1", "sess-1", domain.ControlScope{})

	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("RequestControl() error = %v, want ErrSessionNotFound", err)
	}
}

func TestRequestControl_ApprovalTimeout(t *testing.T) {
	withShrunkControlApprovalTimeout(t, 20*time.Millisecond)
	svc, _, sessions := newTestService()
	sessions.sessionShellFunc = existingSession("sess-1")

	_, err := svc.RequestControl("client-1", "sess-1", domain.ControlScope{})

	if !errors.Is(err, ErrControlApprovalTimeout) {
		t.Fatalf("RequestControl() error = %v, want ErrControlApprovalTimeout", err)
	}
}

func TestRequestControl_AlreadyDelegatedToAnotherClientIsRejectedWithoutPrompt(t *testing.T) {
	svc, pub, sessions := newTestService()
	sessions.sessionShellFunc = existingSession("sess-1")

	// client-1 grabs control first.
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

	eventsBefore := len(pub.all())

	// client-2 requests the same session -- must be rejected immediately,
	// no approval event published.
	_, err := svc.RequestControl("client-2", "sess-1", domain.ControlScope{})

	if !errors.Is(err, ErrSessionAlreadyDelegated) {
		t.Fatalf("RequestControl() error = %v, want ErrSessionAlreadyDelegated", err)
	}
	if len(pub.all()) != eventsBefore {
		t.Error("expected no new approval event to be published for a session already delegated elsewhere")
	}
}

func TestRequestControl_SameClientRepeatIsIdempotent(t *testing.T) {
	svc, pub, sessions := newTestService()
	sessions.sessionShellFunc = existingSession("sess-1")

	var wg sync.WaitGroup
	var first domain.Delegation
	wg.Add(1)
	go func() {
		defer wg.Done()
		first, _ = svc.RequestControl("client-1", "sess-1", domain.ControlScope{})
	}()
	requestID := waitForControlApprovalRequest(t, pub)
	if err := svc.RespondControlApproval(requestID, true); err != nil {
		t.Fatalf("RespondControlApproval() error = %v", err)
	}
	wg.Wait()

	eventsBefore := len(pub.all())
	second, err := svc.RequestControl("client-1", "sess-1", domain.ControlScope{})

	if err != nil {
		t.Fatalf("second RequestControl() error = %v", err)
	}
	if second != first {
		t.Errorf("second RequestControl() = %+v, want same delegation %+v", second, first)
	}
	if len(pub.all()) != eventsBefore {
		t.Error("expected no new approval event for an idempotent repeat request")
	}
}

func TestReleaseControl_Success(t *testing.T) {
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

	if err := svc.ReleaseControl("client-1", "sess-1"); err != nil {
		t.Fatalf("ReleaseControl() error = %v", err)
	}

	svc.mu.Lock()
	_, stillThere := svc.delegations["sess-1"]
	svc.mu.Unlock()
	if stillThere {
		t.Error("expected delegation to be removed after ReleaseControl")
	}
}

func TestReleaseControl_WrongClientIsRejected(t *testing.T) {
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

	if err := svc.ReleaseControl("client-2", "sess-1"); !errors.Is(err, ErrNotYourDelegation) {
		t.Fatalf("ReleaseControl() error = %v, want ErrNotYourDelegation", err)
	}
}

func TestReleaseControl_NotDelegatedIsRejected(t *testing.T) {
	svc, _, _ := newTestService()

	if err := svc.ReleaseControl("client-1", "sess-1"); !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("ReleaseControl() error = %v, want ErrNotDelegated", err)
	}
}

func TestSweepExpired_ReapsOnlyExpiredDelegations(t *testing.T) {
	svc, _, _ := newTestService()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	fresh := domain.NewDelegation("sess-fresh", "client-1", domain.ControlScope{})
	_ = fresh.TransitionTo(domain.DelegActive)
	fresh.LastActAt = now.Add(-1 * time.Minute)

	stale := domain.NewDelegation("sess-stale", "client-1", domain.ControlScope{})
	_ = stale.TransitionTo(domain.DelegActive)
	stale.LastActAt = now.Add(-1 * time.Hour)

	svc.mu.Lock()
	svc.delegations["sess-fresh"] = fresh
	svc.delegations["sess-stale"] = stale
	svc.mu.Unlock()

	reaped := svc.SweepExpired(now)

	if len(reaped) != 1 || reaped[0] != "sess-stale" {
		t.Fatalf("SweepExpired() reaped = %v, want [sess-stale]", reaped)
	}
	svc.mu.Lock()
	_, staleStillThere := svc.delegations["sess-stale"]
	_, freshStillThere := svc.delegations["sess-fresh"]
	svc.mu.Unlock()
	if staleStillThere {
		t.Error("expected sess-stale to be removed")
	}
	if !freshStillThere {
		t.Error("expected sess-fresh to remain")
	}
	if fresh.State != domain.DelegActive {
		t.Errorf("fresh.State = %v, want unchanged %v", fresh.State, domain.DelegActive)
	}
	if stale.State != domain.DelegExpired {
		t.Errorf("stale.State = %v, want %v", stale.State, domain.DelegExpired)
	}
}
