package aicontrol

import (
	"errors"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/service/aicontrol/resolve"
)

func newTestServiceWithResolver(verdict resolve.Verdict) (*Service, *recordingPublisher, *fakeSessionCreator) {
	pub := &recordingPublisher{}
	sessions := &fakeSessionCreator{}
	svc := New(Deps{
		Hosts:     newFakeHostRepo(),
		Sessions:  sessions,
		Publisher: pub,
		Resolver:  &fakeResolver{verdict: verdict},
	})
	return svc, pub, sessions
}

// newTestServiceWithResolverAndAudit is newTestServiceWithResolver plus a
// wired fakeAuditRecorder, for tests asserting E3's emission points. Kept
// separate (rather than changing newTestServiceWithResolver's signature) so
// every other RunCommand test continues to exercise the nil-audit no-op
// path unmodified.
func newTestServiceWithResolverAndAudit(verdict resolve.Verdict) (*Service, *fakeSessionCreator, *fakeAuditRecorder) {
	sessions := &fakeSessionCreator{}
	audit := &fakeAuditRecorder{}
	svc := New(Deps{
		Hosts:     newFakeHostRepo(),
		Sessions:  sessions,
		Publisher: &recordingPublisher{},
		Resolver:  &fakeResolver{verdict: verdict},
		Audit:     audit,
	})
	return svc, sessions, audit
}

// newTestServiceWithResolverAuditAndCapture is newTestServiceWithResolverAndAudit
// plus a wired fakeCapture, for tests asserting E3-b's Begin/End wiring at
// the arm/lifecycle sites. Kept separate for the same reason as that
// helper: every other test continues to exercise the nil-capture no-op path
// unmodified.
func newTestServiceWithResolverAuditAndCapture(verdict resolve.Verdict) (*Service, *fakeSessionCreator, *fakeAuditRecorder, *fakeCapture) {
	sessions := &fakeSessionCreator{}
	audit := &fakeAuditRecorder{}
	capture := &fakeCapture{}
	svc := New(Deps{
		Hosts:     newFakeHostRepo(),
		Sessions:  sessions,
		Publisher: &recordingPublisher{},
		Resolver:  &fakeResolver{verdict: verdict},
		Audit:     audit,
		Capture:   capture,
	})
	return svc, sessions, audit, capture
}

// seedDelegation installs an already-active delegation directly (bypassing
// RequestControl's approval flow -- RunCommand only needs the map entry to
// already exist). lastActAt lets a test assert RunCommand bumps it forward.
func seedDelegation(svc *Service, sessionID, clientID string, lastActAt time.Time) *domain.Delegation {
	deleg := domain.NewDelegation(sessionID, clientID, domain.ControlScope{})
	_ = deleg.TransitionTo(domain.DelegActive)
	deleg.GrantedAt = lastActAt
	deleg.LastActAt = lastActAt

	svc.mu.Lock()
	svc.delegations[sessionID] = deleg
	svc.mu.Unlock()
	return deleg
}

func withShrunkCommandApprovalTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	original := commandApprovalTimeout
	commandApprovalTimeout = d
	t.Cleanup(func() { commandApprovalTimeout = original })
}

func waitForCommandApprovalRequest(t *testing.T, pub *recordingPublisher) commandApprovalPayload {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range pub.all() {
			if payload, ok := e.payload.(commandApprovalPayload); ok {
				return payload
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for mcp:cmd-approval event")
	return commandApprovalPayload{}
}

func TestRunCommand_AutoRunsLowRiskCertainCommand(t *testing.T) {
	svc, pub, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	past := time.Now().Add(-time.Hour)
	seedDelegation(svc, "sess-1", "client-1", past)

	handle, err := svc.RunCommand("client-1", "sess-1", "echo hi")

	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	want := domain.CommandHandle{SessionID: "sess-1", Command: "echo hi", State: domain.CmdRunning, Seq: 1}
	if handle != want {
		t.Errorf("RunCommand() = %+v, want %+v", handle, want)
	}
	writes := sessions.allWrites()
	if len(writes) != 1 || string(writes[0].data) != "echo hi\r" || writes[0].sessionID != "sess-1" {
		t.Errorf("writes = %+v, want one write of %q to sess-1", writes, "echo hi\r")
	}
	for _, e := range pub.all() {
		if _, ok := e.payload.(commandApprovalPayload); ok {
			t.Error("expected no mcp:cmd-approval event for an auto-runnable command")
		}
	}
	if len(pub.all()) != 1 {
		t.Errorf("expected exactly one mcp:cmd-state event (initial running), got %d events", len(pub.all()))
	}

	svc.mu.Lock()
	lastActAt := svc.delegations["sess-1"].LastActAt
	svc.mu.Unlock()
	if !lastActAt.After(past) {
		t.Error("expected LastActAt to be bumped forward by a successful RunCommand")
	}
}

func TestRunCommand_ApprovedRiskyCommandInjectsGuardedForm(t *testing.T) {
	verdict := resolve.Verdict{Risk: resolve.RiskMedium, Reasons: []string{"destructive verb, variable target"}, GuardedCmd: "if [ -n \"$X\" ]; then rm -rf \"$X\"; else echo momo:abort; fi"}
	svc, pub, sessions := newTestServiceWithResolver(verdict)
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	var wg sync.WaitGroup
	var handle domain.CommandHandle
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		handle, reqErr = svc.RunCommand("client-1", "sess-1", "rm -rf $X")
	}()

	payload := waitForCommandApprovalRequest(t, pub)
	if payload.Risk != "medium" || payload.GuardedCmd != verdict.GuardedCmd || payload.Command != "rm -rf $X" {
		t.Errorf("approval payload = %+v, want risk=medium, original command, guardedCmd=%q", payload, verdict.GuardedCmd)
	}
	if err := svc.RespondCommandApproval(payload.RequestID, true); err != nil {
		t.Fatalf("RespondCommandApproval() error = %v", err)
	}
	wg.Wait()

	if reqErr != nil {
		t.Fatalf("RunCommand() error = %v", reqErr)
	}
	if handle.Command != verdict.GuardedCmd {
		t.Errorf("handle.Command = %q, want the guarded form %q", handle.Command, verdict.GuardedCmd)
	}
	if handle.State != domain.CmdRunning || handle.Seq != 1 {
		t.Errorf("handle = %+v, want State=running Seq=1 armed after injection", handle)
	}
	writes := sessions.allWrites()
	if len(writes) != 1 || string(writes[0].data) != verdict.GuardedCmd+"\r" {
		t.Errorf("writes = %+v, want one write of the guarded form", writes)
	}
}

func TestRunCommand_DenyReturnsErrCommandDenied(t *testing.T) {
	svc, pub, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskHigh})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	var wg sync.WaitGroup
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, reqErr = svc.RunCommand("client-1", "sess-1", "rm -rf /")
	}()

	payload := waitForCommandApprovalRequest(t, pub)
	if err := svc.RespondCommandApproval(payload.RequestID, false); err != nil {
		t.Fatalf("RespondCommandApproval() error = %v", err)
	}
	wg.Wait()

	if !errors.Is(reqErr, ErrCommandDenied) {
		t.Fatalf("RunCommand() error = %v, want ErrCommandDenied", reqErr)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write to the session for a denied command")
	}
}

func TestRunCommand_ApprovalTimeout(t *testing.T) {
	withShrunkCommandApprovalTimeout(t, 20*time.Millisecond)
	svc, _, sessions := newTestServiceWithResolver(resolve.Verdict{Uncertain: true})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	_, err := svc.RunCommand("client-1", "sess-1", "$(some_func)")

	if !errors.Is(err, ErrCommandApprovalTimeout) {
		t.Fatalf("RunCommand() error = %v, want ErrCommandApprovalTimeout", err)
	}
}

func TestRunCommand_NotDelegatedIsRejected(t *testing.T) {
	svc, _, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")

	_, err := svc.RunCommand("client-1", "sess-1", "echo hi")

	if !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("RunCommand() error = %v, want ErrNotDelegated", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write for an undelegated session")
	}
}

func TestRunCommand_WrongClientIsRejected(t *testing.T) {
	svc, _, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	_, err := svc.RunCommand("client-2", "sess-1", "echo hi")

	if !errors.Is(err, ErrNotYourDelegation) {
		t.Fatalf("RunCommand() error = %v, want ErrNotYourDelegation", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write when the delegation belongs to a different client")
	}
}

func TestRunCommand_SessionGoneIsRejected(t *testing.T) {
	svc, _, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("some-other-session") // sess-1 no longer live
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	_, err := svc.RunCommand("client-1", "sess-1", "echo hi")

	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("RunCommand() error = %v, want ErrSessionNotFound", err)
	}
}

func TestRunCommand_RecordsAuditOnAutoRun(t *testing.T) {
	svc, sessions, audit := newTestServiceWithResolverAndAudit(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "echo hi"); err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}

	events := audit.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	e := events[0]
	if e.Kind != domain.AuditKindCommand || e.Decision != "auto" || e.Approver != "" || e.OriginalCmd != "echo hi" || e.SessionID != "sess-1" {
		t.Errorf("audit event = %+v, want kind=command decision=auto approver=\"\"", e)
	}
}

func TestRunCommand_RecordsAuditOnApproved(t *testing.T) {
	verdict := resolve.Verdict{Risk: resolve.RiskMedium, GuardedCmd: "guarded"}
	svc, sessions, audit := newTestServiceWithResolverAndAudit(verdict)
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.RunCommand("client-1", "sess-1", "rm -rf $X")
	}()
	deadline := time.Now().Add(2 * time.Second)
	var requestID string
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		for id := range svc.pending {
			requestID = id
		}
		svc.mu.Unlock()
		if requestID != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("timed out waiting for a pending command approval")
	}
	if err := svc.RespondCommandApproval(requestID, true); err != nil {
		t.Fatalf("RespondCommandApproval() error = %v", err)
	}
	wg.Wait()

	events := audit.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	e := events[0]
	if e.Decision != "approved" || e.Approver != "custodian" || e.GuardedCmd != "guarded" {
		t.Errorf("audit event = %+v, want decision=approved approver=custodian guardedCmd=guarded", e)
	}
}

func TestRunCommand_RecordsAuditOnDenied(t *testing.T) {
	svc, sessions, audit := newTestServiceWithResolverAndAudit(resolve.Verdict{Risk: resolve.RiskHigh})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.RunCommand("client-1", "sess-1", "rm -rf /")
	}()
	deadline := time.Now().Add(2 * time.Second)
	var requestID string
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		for id := range svc.pending {
			requestID = id
		}
		svc.mu.Unlock()
		if requestID != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("timed out waiting for a pending command approval")
	}
	if err := svc.RespondCommandApproval(requestID, false); err != nil {
		t.Fatalf("RespondCommandApproval() error = %v", err)
	}
	wg.Wait()

	events := audit.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if e := events[0]; e.Decision != "rejected" || e.Approver != "custodian" {
		t.Errorf("audit event = %+v, want decision=rejected approver=custodian", e)
	}
}

func TestRunCommand_RecordsAuditOnApprovalTimeout(t *testing.T) {
	withShrunkCommandApprovalTimeout(t, 20*time.Millisecond)
	svc, sessions, audit := newTestServiceWithResolverAndAudit(resolve.Verdict{Uncertain: true})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "$(some_func)"); !errors.Is(err, ErrCommandApprovalTimeout) {
		t.Fatalf("RunCommand() error = %v, want ErrCommandApprovalTimeout", err)
	}

	events := audit.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if e := events[0]; e.Decision != "timeout" || e.Approver != "custodian" {
		t.Errorf("audit event = %+v, want decision=timeout approver=custodian", e)
	}
}

// TestRunCommand_ArmsCaptureWithAuditID confirms E3-b's capture tap is
// begun at the same point the command handle is armed, linked to the exact
// audit row RunCommand just recorded for the executed decision.
func TestRunCommand_ArmsCaptureWithAuditID(t *testing.T) {
	svc, sessions, audit, capture := newTestServiceWithResolverAuditAndCapture(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "echo hi"); err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}

	events := audit.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	wantID := events[0].ID

	begins := capture.allBegins()
	if len(begins) != 1 {
		t.Fatalf("capture.Begin calls = %d, want 1", len(begins))
	}
	if begins[0].sessionID != "sess-1" || begins[0].auditID != wantID {
		t.Errorf("capture.Begin(%q, %d), want (\"sess-1\", %d)", begins[0].sessionID, begins[0].auditID, wantID)
	}
}

// TestRunCommand_DeniedCommandNeverArmsCapture confirms a rejected command
// never begins a capture -- RunCommand returns before the arm site for
// every non-executed decision (timeout/rejected).
func TestRunCommand_DeniedCommandNeverArmsCapture(t *testing.T) {
	svc, sessions, _, capture := newTestServiceWithResolverAuditAndCapture(resolve.Verdict{Risk: resolve.RiskHigh})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.RunCommand("client-1", "sess-1", "rm -rf /")
	}()
	deadline := time.Now().Add(2 * time.Second)
	var requestID string
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		for id := range svc.pending {
			requestID = id
		}
		svc.mu.Unlock()
		if requestID != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("timed out waiting for a pending command approval")
	}
	if err := svc.RespondCommandApproval(requestID, false); err != nil {
		t.Fatalf("RespondCommandApproval() error = %v", err)
	}
	wg.Wait()

	if begins := capture.allBegins(); len(begins) != 0 {
		t.Errorf("capture.Begin calls = %d, want 0 for a denied command", len(begins))
	}
}

func TestDialectFromShell(t *testing.T) {
	cases := []struct {
		name  string
		shell string
		want  string
	}{
		{"ssh has no recorded shell", "", ""},
		{"absolute bash path", "/bin/bash", "bash"},
		{"bash on PATH", "bash", "bash"},
		{"posix sh", "/bin/sh", "sh"},
		{"dash", "/usr/bin/dash", "sh"},
		{"powershell passed through lowercased", "powershell.exe", "powershell.exe"},
		{"pwsh passed through lowercased", `C:\Program Files\PowerShell\7\pwsh.exe`, "pwsh.exe"},
		{"cmd passed through", "cmd.exe", "cmd.exe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dialectFromShell(c.shell); got != c.want {
				t.Errorf("dialectFromShell(%q) = %q, want %q", c.shell, got, c.want)
			}
		})
	}
}
