package aicontrol

import (
	"testing"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/service/aicontrol/resolve"
	"momo-shell/internal/core/service/shellintegration"
)

// var _ shellintegration.Observer = (*Service)(nil) proves Service's method
// set satisfies the interface F3's AddObserver expects -- production
// registration is A7's job (composition root), so this compile-time
// assertion is the only place that conformance is checked today.
var _ shellintegration.Observer = (*Service)(nil)

func TestOnCommandEnd_TransitionsArmedHandleToDone(t *testing.T) {
	svc, pub, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "echo hi"); err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	before := len(pub.all())

	svc.OnCommandEnd("sess-1", 0)

	svc.mu.Lock()
	h := *svc.commands["sess-1"]
	svc.mu.Unlock()
	if h.State != domain.CmdDone || h.ExitCode == nil || *h.ExitCode != 0 || h.Seq != 2 {
		t.Errorf("handle = %+v, want {State:done, ExitCode:0, Seq:2}", h)
	}

	events := pub.all()[before:]
	if len(events) != 1 {
		t.Fatalf("expected exactly one new mcp:cmd-state event, got %d", len(events))
	}
	payload, ok := events[0].payload.(commandStatePayload)
	if !ok {
		t.Fatalf("event payload = %+v, want commandStatePayload", events[0].payload)
	}
	if payload.State != "done" || payload.ExitCode == nil || *payload.ExitCode != 0 || payload.Seq != 2 {
		t.Errorf("payload = %+v, want {State:done, ExitCode:0, Seq:2}", payload)
	}
}

func TestOnAltScreen_CyclesRunningAndTUI(t *testing.T) {
	svc, pub, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "vim file.txt"); err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	before := len(pub.all())

	svc.OnAltScreen("sess-1", true)  // enter alt-screen -> tui
	svc.OnAltScreen("sess-1", false) // exit alt-screen -> running
	svc.OnCommandEnd("sess-1", 0)    // vim exits -> done

	events := pub.all()[before:]
	if len(events) != 3 {
		t.Fatalf("expected 3 mcp:cmd-state events (tui, running, done), got %d", len(events))
	}
	wantStates := []string{"tui", "running", "done"}
	wantSeqs := []int{2, 3, 4}
	for i, e := range events {
		p, ok := e.payload.(commandStatePayload)
		if !ok {
			t.Fatalf("event[%d] payload = %+v, want commandStatePayload", i, e.payload)
		}
		if p.State != wantStates[i] || p.Seq != wantSeqs[i] {
			t.Errorf("event[%d] = %+v, want State=%s Seq=%d", i, p, wantStates[i], wantSeqs[i])
		}
	}
}

func TestOnAltScreen_DuplicateSignalIsIgnored(t *testing.T) {
	svc, pub, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "echo hi"); err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	before := len(pub.all())

	svc.OnAltScreen("sess-1", false) // already running -- duplicate "exit" signal

	if got := len(pub.all()) - before; got != 0 {
		t.Errorf("expected no event for a no-op alt-screen signal, got %d", got)
	}
}

func TestLifecycleCallbacks_IgnoreUnknownSession(t *testing.T) {
	svc, pub, _ := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})

	svc.OnCommandEnd("no-such-session", 1)
	svc.OnAltScreen("no-such-session", true)
	svc.OnCommandStart("no-such-session")
	svc.OnPrompt("no-such-session")

	if len(pub.all()) != 0 {
		t.Errorf("expected no event for a session with no armed command, got %d", len(pub.all()))
	}
}

// TestOnCommandEnd_EndsCapture confirms OnCommandEnd flags the E3-b capture
// as finished (lifecycle.go's End call). The capture package's own tests
// cover the actual seal-on-next-OnOutput behavior; this only asserts the
// wiring reaches it with the right sessionID.
func TestOnCommandEnd_EndsCapture(t *testing.T) {
	svc, sessions, _, capture := newTestServiceWithResolverAuditAndCapture(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "echo hi"); err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}

	svc.OnCommandEnd("sess-1", 0)

	if ends := capture.allEnds(); len(ends) != 1 || ends[0] != "sess-1" {
		t.Errorf("capture.End calls = %+v, want [\"sess-1\"]", ends)
	}
}

func TestLifecycleCallbacks_IgnoreAlreadyDoneSession(t *testing.T) {
	svc, pub, sessions := newTestServiceWithResolver(resolve.Verdict{Risk: resolve.RiskLow})
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.RunCommand("client-1", "sess-1", "echo hi"); err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	svc.OnCommandEnd("sess-1", 0)
	before := len(pub.all())

	svc.OnCommandEnd("sess-1", 1)   // late duplicate exit -- ignored
	svc.OnAltScreen("sess-1", true) // late alt-screen -- ignored

	if got := len(pub.all()) - before; got != 0 {
		t.Errorf("expected no event once a command is done, got %d", got)
	}
	svc.mu.Lock()
	h := *svc.commands["sess-1"]
	svc.mu.Unlock()
	if h.State != domain.CmdDone || *h.ExitCode != 0 {
		t.Errorf("handle = %+v, want the first done transition to stick", h)
	}
}
