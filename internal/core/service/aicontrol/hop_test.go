package aicontrol

import (
	"testing"
	"time"

	"momo-shell/internal/adapter/out/shparse"
	"momo-shell/internal/core/port/out"
	"momo-shell/internal/core/service/aicontrol/resolve"
)

func TestDetectHop(t *testing.T) {
	cases := []struct {
		name     string
		commands []out.SimpleCommand
		wantVerb string
		wantHit  bool
	}{
		{"ssh to a host", []out.SimpleCommand{cmd("ssh", "host")}, "ssh", true},
		{"su to another user", []out.SimpleCommand{cmd("su", "-")}, "su", true},
		{"sudo -i opens an interactive shell", []out.SimpleCommand{cmd("sudo", "-i")}, "sudo", true},
		{"sudo -s opens an interactive shell", []out.SimpleCommand{cmd("sudo", "-s")}, "sudo", true},
		{"sudo -is combined flags", []out.SimpleCommand{cmd("sudo", "-is")}, "sudo", true},
		{"plain sudo cmd is not a hop", []out.SimpleCommand{cmd("sudo", "rm", "-rf", "/")}, "", false},
		{"docker exec enters a container", []out.SimpleCommand{cmd("docker", "exec", "-it", "c", "bash")}, "docker", true},
		{"docker ps is not a hop", []out.SimpleCommand{cmd("docker", "ps")}, "", false},
		{"kubectl exec enters a container", []out.SimpleCommand{cmd("kubectl", "exec", "pod", "--", "sh")}, "kubectl", true},
		{"kubectl get pods is not a hop", []out.SimpleCommand{cmd("kubectl", "get", "pods")}, "", false},
		{"pipeline matches the hop stage", []out.SimpleCommand{cmd("cat", "x"), cmd("ssh", "host")}, "ssh", true},
		{"plain non-hop command", []out.SimpleCommand{cmd("echo", "hi")}, "", false},
		{"empty analysis (parse failure) has no match", nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hint, ok := detectHop(out.CommandAnalysis{Commands: tc.commands})
			if ok != tc.wantHit {
				t.Fatalf("detectHop() ok = %v, want %v (hint=%+v)", ok, tc.wantHit, hint)
			}
			if ok && hint.Verb != tc.wantVerb {
				t.Errorf("hint.Verb = %q, want %q", hint.Verb, tc.wantVerb)
			}
			if ok && hint.Reason == "" {
				t.Error("expected a non-empty Reason when a hint is returned")
			}
		})
	}
}

func TestRunCommand_HopVerbDemotedToApproval(t *testing.T) {
	verdict := resolve.Verdict{
		Risk:      resolve.RiskLow,
		Uncertain: false,
		Analysis:  out.CommandAnalysis{Commands: []out.SimpleCommand{cmd("ssh", "host")}},
	}
	svc, pub, sessions := newTestServiceWithResolver(verdict)
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	go func() {
		_, _ = svc.RunCommand("client-1", "sess-1", "ssh host")
	}()

	payload := waitForCommandApprovalRequest(t, pub)
	found := false
	for _, r := range payload.Reasons {
		if r != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("approval payload.Reasons = %v, want a hop reason", payload.Reasons)
	}
	if err := svc.RespondCommandApproval(payload.RequestID, true); err != nil {
		t.Fatalf("RespondCommandApproval() error = %v", err)
	}
}

func TestRunCommand_RecordsHopDemotionInAudit(t *testing.T) {
	verdict := resolve.Verdict{
		Risk:      resolve.RiskLow,
		Uncertain: false,
		Analysis:  out.CommandAnalysis{Commands: []out.SimpleCommand{cmd("ssh", "host")}},
	}
	svc, sessions, audit := newTestServiceWithResolverAndAudit(verdict)
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	done := make(chan error, 1)
	go func() {
		_, err := svc.RunCommand("client-1", "sess-1", "ssh host")
		done <- err
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
	if err := <-done; err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}

	events := audit.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	e := events[0]
	if e.Decision != "approved" || e.Approver != "custodian" {
		t.Errorf("audit event = %+v, want decision=approved approver=custodian", e)
	}
	found := false
	for _, r := range e.Resolve.Reasons {
		if r != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("audit event Resolve.Reasons = %v, want a hop reason", e.Resolve.Reasons)
	}
}

// --- Integration smoke test: the real shparse + resolve pipeline, not a
// fake resolver -- proves sudo/ssh tokenize as a plain Verb (no unwrapping)
// and the hop gate demotes to approval rather than rejecting or auto-running
// (contrast TestRunCommand_Integration_TopIsRejectedBeforeInjection).

func TestRunCommand_Integration_SSHDemotedBeforeInjection(t *testing.T) {
	for _, command := range []string{"ssh host", "sudo -i"} {
		t.Run(command, func(t *testing.T) {
			resolver := resolve.New(resolve.Deps{Parser: shparse.New()})
			pub := &recordingPublisher{}
			sessions := &fakeSessionCreator{sessionShellFunc: existingSession("sess-1")}
			svc := New(Deps{
				Hosts:     newFakeHostRepo(),
				Sessions:  sessions,
				Publisher: pub,
				Resolver:  resolver,
			})
			seedDelegation(svc, "sess-1", "client-1", time.Now().Add(-time.Hour))

			go func() {
				_, _ = svc.RunCommand("client-1", "sess-1", command)
			}()

			payload := waitForCommandApprovalRequest(t, pub)
			if payload.Risk != "low" || payload.Uncertain {
				t.Errorf("payload = %+v, want a low-risk/certain verdict demoted by the hop gate", payload)
			}
			if len(sessions.allWrites()) != 0 {
				t.Error("expected no write before approval")
			}
			if err := svc.RespondCommandApproval(payload.RequestID, true); err != nil {
				t.Fatalf("RespondCommandApproval() error = %v", err)
			}
		})
	}
}
