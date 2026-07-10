package aicontrol

import (
	"errors"
	"sync"
	"testing"

	"momo-shell/internal/core/domain"
)

// TestKillControl_DelegatedUserSession_InterruptsWithoutClosing covers the
// RequestControl-borrowed path (doc 21 §K2): KillControl must revoke the
// delegation, clear any in-flight command handle, send Ctrl-C to interrupt
// the running command, and never Close the user's own shell.
func TestKillControl_DelegatedUserSession_InterruptsWithoutClosing(t *testing.T) {
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

	// Simulate an in-flight command (RunCommand's bookkeeping) so KillControl's
	// cleanup of s.commands can be observed.
	svc.mu.Lock()
	svc.commands["sess-1"] = domain.NewCommandHandle("sess-1", "sleep 100")
	svc.mu.Unlock()

	if err := svc.KillControl("sess-1"); err != nil {
		t.Fatalf("KillControl() error = %v", err)
	}

	svc.mu.Lock()
	_, delegStillThere := svc.delegations["sess-1"]
	_, cmdStillThere := svc.commands["sess-1"]
	svc.mu.Unlock()
	if delegStillThere {
		t.Error("expected delegation to be removed after KillControl")
	}
	if cmdStillThere {
		t.Error("expected in-flight command handle to be removed after KillControl")
	}

	writes := sessions.allWrites()
	if len(writes) != 1 || writes[0].sessionID != "sess-1" || string(writes[0].data) != "\x03" {
		t.Fatalf("allWrites() = %+v, want a single Ctrl-C write to sess-1", writes)
	}
	if closes := sessions.allCloses(); len(closes) != 0 {
		t.Errorf("allCloses() = %v, want none -- a delegated user session must not be closed", closes)
	}
}

// TestKillControl_AICreatedSession_ClosesEntirely covers the ConnectHost
// path (doc 21 §K2): the session is disposable, so KillControl closes it
// entirely instead of merely interrupting it.
func TestKillControl_AICreatedSession_ClosesEntirely(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, sessions := newTestService(host)

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

	if err := svc.KillControl("sess-1"); err != nil {
		t.Fatalf("KillControl() error = %v", err)
	}

	svc.mu.Lock()
	_, delegStillThere := svc.delegations["sess-1"]
	svc.mu.Unlock()
	if delegStillThere {
		t.Error("expected delegation to be removed after KillControl")
	}

	closes := sessions.allCloses()
	if len(closes) != 1 || closes[0] != "sess-1" {
		t.Fatalf("allCloses() = %v, want [sess-1]", closes)
	}
	if writes := sessions.allWrites(); len(writes) != 0 {
		t.Errorf("allWrites() = %+v, want none -- an AI-created session must be closed, not interrupted", writes)
	}
}

// TestKillControl_NotDelegatedIsRejected confirms KillControl on a session
// with no active delegation is rejected rather than silently no-op'ing.
func TestKillControl_NotDelegatedIsRejected(t *testing.T) {
	svc, _, _ := newTestService()

	if err := svc.KillControl("sess-1"); !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("KillControl() error = %v, want ErrNotDelegated", err)
	}
}

// TestKillControl_CleanupErrorIsNotFatal confirms a failing Close still
// leaves the delegation revoked and KillControl reporting success -- the
// revoke is the safety guarantee (doc 21 §K1), cleanup is best-effort.
func TestKillControl_CleanupErrorIsNotFatal(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, sessions := newTestService(host)
	sessions.closeFunc = func(sessionID string) error { return errors.New("close failed") }

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

	if err := svc.KillControl("sess-1"); err != nil {
		t.Fatalf("KillControl() error = %v, want nil (cleanup errors are non-fatal)", err)
	}

	svc.mu.Lock()
	_, delegStillThere := svc.delegations["sess-1"]
	svc.mu.Unlock()
	if delegStillThere {
		t.Error("expected delegation to be removed even though cleanup failed")
	}
}

// TestKillControl_RecordsAudit confirms the custodian's emergency stop
// (US-1) is itself an audited decision.
func TestKillControl_RecordsAudit(t *testing.T) {
	svc, pub, sessions, audit := newTestServiceForControlWithAudit()
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

	if err := svc.KillControl("sess-1"); err != nil {
		t.Fatalf("KillControl() error = %v", err)
	}

	events := audit.all()
	if len(events) != 2 { // granted, then killed
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	if e := events[1]; e.Decision != "killed" || e.Approver != "custodian" || e.ClientID != "client-1" {
		t.Errorf("audit event = %+v, want decision=killed approver=custodian clientID=client-1", e)
	}
}

// TestKillControl_OriginFlag confirms ConnectHost marks its delegation
// AICreated and RequestControl does not -- KillControl's routing depends on
// this flag being set correctly by each grant path.
func TestKillControl_OriginFlag(t *testing.T) {
	host := domain.Host{ID: "host-1", Name: "web-1"}
	svc, pub, sessions := newTestService(host)
	sessions.sessionShellFunc = existingSession("sess-borrowed")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = svc.ConnectHost("client-1", "web-1")
	}()
	connectReqID := waitForConnectApprovalRequest(t, pub)
	if err := svc.RespondConnectApproval(connectReqID, true); err != nil {
		t.Fatalf("RespondConnectApproval() error = %v", err)
	}

	go func() {
		defer wg.Done()
		_, _ = svc.RequestControl("client-2", "sess-borrowed", domain.ControlScope{})
	}()
	controlReqID := waitForControlApprovalRequest(t, pub)
	if err := svc.RespondControlApproval(controlReqID, true); err != nil {
		t.Fatalf("RespondControlApproval() error = %v", err)
	}
	wg.Wait()

	svc.mu.Lock()
	connectDeleg := svc.delegations["sess-1"]
	controlDeleg := svc.delegations["sess-borrowed"]
	svc.mu.Unlock()

	if connectDeleg == nil || !connectDeleg.AICreated {
		t.Errorf("ConnectHost delegation.AICreated = %+v, want true", connectDeleg)
	}
	if controlDeleg == nil || controlDeleg.AICreated {
		t.Errorf("RequestControl delegation.AICreated = %+v, want false", controlDeleg)
	}
}
