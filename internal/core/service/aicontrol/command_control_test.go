package aicontrol

import (
	"errors"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

// TestCancelCommand_WritesInterruptAndReturnsRunningSnapshot covers the
// happy path (doc 18 D2): a single Ctrl-C write, no FSM transition/publish
// (the shell-integration Observer drives done asynchronously with the real
// exit code, mirrored below).
func TestCancelCommand_WritesInterruptAndReturnsRunningSnapshot(t *testing.T) {
	svc, pub, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	svc.commands["sess-1"] = domain.NewCommandHandle("sess-1", "sleep 100")

	h, err := svc.CancelCommand("client-1", "sess-1")
	if err != nil {
		t.Fatalf("CancelCommand() error = %v", err)
	}
	if h.State != domain.CmdRunning {
		t.Errorf("returned snapshot State = %q, want %q (cancel doesn't transition the FSM itself)", h.State, domain.CmdRunning)
	}

	writes := sessions.allWrites()
	if len(writes) != 1 || writes[0].sessionID != "sess-1" || string(writes[0].data) != "\x03" {
		t.Fatalf("allWrites() = %+v, want a single Ctrl-C write to sess-1", writes)
	}
	if len(pub.all()) != 0 {
		t.Error("expected no mcp:cmd-state event from CancelCommand itself")
	}

	// Simulate the shell-integration Observer landing the interrupt as a
	// real exit code, same as KillControl's Ctrl-C path.
	svc.OnCommandEnd("sess-1", 130)
	svc.mu.Lock()
	done := svc.commands["sess-1"]
	svc.mu.Unlock()
	if done.State != domain.CmdDone || done.ExitCode == nil || *done.ExitCode != 130 {
		t.Errorf("after OnCommandEnd(130): handle = %+v, want State=done ExitCode=130", done)
	}
}

func TestCancelCommand_NotDelegatedRejected(t *testing.T) {
	svc, _, sessions := newTestService()

	if _, err := svc.CancelCommand("client-1", "sess-1"); !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("CancelCommand() error = %v, want ErrNotDelegated", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write without a delegation")
	}
}

func TestCancelCommand_NotYourDelegationRejected(t *testing.T) {
	svc, _, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	svc.commands["sess-1"] = domain.NewCommandHandle("sess-1", "sleep 100")

	if _, err := svc.CancelCommand("client-2", "sess-1"); !errors.Is(err, ErrNotYourDelegation) {
		t.Fatalf("CancelCommand() error = %v, want ErrNotYourDelegation", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write for another client's delegation")
	}
}

func TestCancelCommand_NoActiveCommandRejected(t *testing.T) {
	svc, _, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.CancelCommand("client-1", "sess-1"); !errors.Is(err, ErrNoActiveCommand) {
		t.Fatalf("CancelCommand() error = %v, want ErrNoActiveCommand", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write without an in-flight command")
	}
}

// TestBackgroundCommand_WritesSuspendAndResumeThenTransitions covers the
// happy path: Ctrl-Z then `bg`, a CmdBackground snapshot, exactly one
// mcp:cmd-state publish, and immunity to a subsequent spurious OnCommandEnd
// (the IsTerminal guard).
func TestBackgroundCommand_WritesSuspendAndResumeThenTransitions(t *testing.T) {
	svc, pub, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	svc.commands["sess-1"] = domain.NewCommandHandle("sess-1", "sleep 100")

	h, err := svc.BackgroundCommand("client-1", "sess-1")
	if err != nil {
		t.Fatalf("BackgroundCommand() error = %v", err)
	}
	if h.State != domain.CmdBackground {
		t.Errorf("returned snapshot State = %q, want %q", h.State, domain.CmdBackground)
	}

	writes := sessions.allWrites()
	if len(writes) != 2 || string(writes[0].data) != "\x1a" || string(writes[1].data) != "bg\r" {
		t.Fatalf("allWrites() = %+v, want [\\x1a, bg\\r]", writes)
	}

	events := pub.all()
	if len(events) != 1 || events[0].topic != "mcp:cmd-state" {
		t.Fatalf("events = %+v, want exactly one mcp:cmd-state event", events)
	}
	payload, ok := events[0].payload.(commandStatePayload)
	if !ok || payload.State != string(domain.CmdBackground) {
		t.Errorf("event payload = %+v, want State=%q", events[0].payload, domain.CmdBackground)
	}

	// A spurious OnCommandEnd (e.g. the `bg` builtin's own prompt-return
	// signal) must not overwrite the backgrounded handle.
	svc.OnCommandEnd("sess-1", 0)
	svc.mu.Lock()
	still := svc.commands["sess-1"]
	svc.mu.Unlock()
	if still.State != domain.CmdBackground {
		t.Errorf("after spurious OnCommandEnd: State = %q, want still %q", still.State, domain.CmdBackground)
	}
}

func TestBackgroundCommand_NotDelegatedRejected(t *testing.T) {
	svc, _, sessions := newTestService()

	if _, err := svc.BackgroundCommand("client-1", "sess-1"); !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("BackgroundCommand() error = %v, want ErrNotDelegated", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write without a delegation")
	}
}

func TestBackgroundCommand_NotYourDelegationRejected(t *testing.T) {
	svc, _, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	svc.commands["sess-1"] = domain.NewCommandHandle("sess-1", "sleep 100")

	if _, err := svc.BackgroundCommand("client-2", "sess-1"); !errors.Is(err, ErrNotYourDelegation) {
		t.Fatalf("BackgroundCommand() error = %v, want ErrNotYourDelegation", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write for another client's delegation")
	}
}

func TestBackgroundCommand_NoActiveCommandRejected(t *testing.T) {
	svc, _, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if _, err := svc.BackgroundCommand("client-1", "sess-1"); !errors.Is(err, ErrNoActiveCommand) {
		t.Fatalf("BackgroundCommand() error = %v, want ErrNoActiveCommand", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write without an in-flight command")
	}
}
