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
	if handle != (domain.CommandHandle{SessionID: "sess-1", Command: "echo hi"}) {
		t.Errorf("RunCommand() = %+v, want {sess-1, echo hi}", handle)
	}
	writes := sessions.allWrites()
	if len(writes) != 1 || string(writes[0].data) != "echo hi\r" || writes[0].sessionID != "sess-1" {
		t.Errorf("writes = %+v, want one write of %q to sess-1", writes, "echo hi\r")
	}
	if len(pub.all()) != 0 {
		t.Error("expected no mcp:cmd-approval event for an auto-runnable command")
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
