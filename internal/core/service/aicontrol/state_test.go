package aicontrol

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetShellState_ReturnsCwdAndEnv(t *testing.T) {
	svc, pub, _ := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	svc.shellState = &fakeShellStateReader{
		readVarsFunc: func(ctx context.Context, sessionID string, names []string) (map[string]string, error) {
			return map[string]string{"PWD": "/home/x", "HOME": "/home/x", "USER": "x"}, nil
		},
	}

	state, err := svc.GetShellState("client-1", "sess-1")
	if err != nil {
		t.Fatalf("GetShellState() error = %v", err)
	}
	if state.Cwd != "/home/x" {
		t.Errorf("state.Cwd = %q, want /home/x", state.Cwd)
	}
	if state.Env["HOME"] != "/home/x" || state.Env["USER"] != "x" {
		t.Errorf("state.Env = %+v, want HOME=/home/x USER=x", state.Env)
	}

	events := pub.all()
	if len(events) != 1 || events[0].topic != "mcp:shell-state" {
		t.Fatalf("events = %+v, want exactly one mcp:shell-state event", events)
	}
}

func TestGetShellState_OmitsUnsetEnv(t *testing.T) {
	svc, _, _ := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	svc.shellState = &fakeShellStateReader{
		readVarsFunc: func(ctx context.Context, sessionID string, names []string) (map[string]string, error) {
			return map[string]string{"PWD": "/home/x"}, nil
		},
	}

	state, err := svc.GetShellState("client-1", "sess-1")
	if err != nil {
		t.Fatalf("GetShellState() error = %v", err)
	}
	if state.Cwd != "/home/x" {
		t.Errorf("state.Cwd = %q, want /home/x", state.Cwd)
	}
	if len(state.Env) != 0 {
		t.Errorf("state.Env = %+v, want empty", state.Env)
	}
}

func TestGetShellState_NotDelegatedRejected(t *testing.T) {
	svc, pub, _ := newTestService()
	svc.shellState = &fakeShellStateReader{
		readVarsFunc: func(ctx context.Context, sessionID string, names []string) (map[string]string, error) {
			t.Fatal("probe should not be called without a delegation")
			return nil, nil
		},
	}

	_, err := svc.GetShellState("client-1", "sess-1")
	if !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("GetShellState() error = %v, want ErrNotDelegated", err)
	}
	if len(pub.all()) != 0 {
		t.Error("expected no mcp:shell-state event without a delegation")
	}
}

func TestGetShellState_NotYourDelegationRejected(t *testing.T) {
	svc, pub, _ := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	svc.shellState = &fakeShellStateReader{
		readVarsFunc: func(ctx context.Context, sessionID string, names []string) (map[string]string, error) {
			t.Fatal("probe should not be called for another client's delegation")
			return nil, nil
		},
	}

	_, err := svc.GetShellState("client-2", "sess-1")
	if !errors.Is(err, ErrNotYourDelegation) {
		t.Fatalf("GetShellState() error = %v, want ErrNotYourDelegation", err)
	}
	if len(pub.all()) != 0 {
		t.Error("expected no mcp:shell-state event for another client's delegation")
	}
}

func TestGetShellState_ProbeErrorPropagates(t *testing.T) {
	svc, pub, _ := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())
	probeErr := errors.New("probe: not at prompt")
	svc.shellState = &fakeShellStateReader{
		readVarsFunc: func(ctx context.Context, sessionID string, names []string) (map[string]string, error) {
			return nil, probeErr
		},
	}

	_, err := svc.GetShellState("client-1", "sess-1")
	if !errors.Is(err, probeErr) {
		t.Fatalf("GetShellState() error = %v, want %v", err, probeErr)
	}
	if len(pub.all()) != 0 {
		t.Error("expected no mcp:shell-state event when the probe errors")
	}
}

func TestResetShell_WritesResetSequence(t *testing.T) {
	svc, _, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if err := svc.ResetShell("client-1", "sess-1"); err != nil {
		t.Fatalf("ResetShell() error = %v", err)
	}

	writes := sessions.allWrites()
	if len(writes) != 1 || writes[0].sessionID != "sess-1" {
		t.Fatalf("writes = %+v, want exactly one write to sess-1", writes)
	}
	if got := string(writes[0].data); got != "\x03cd ~\r" {
		t.Errorf("write data = %q, want %q", got, "\x03cd ~\r")
	}
}

func TestResetShell_NotDelegatedRejected(t *testing.T) {
	svc, _, sessions := newTestService()

	if err := svc.ResetShell("client-1", "sess-1"); !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("ResetShell() error = %v, want ErrNotDelegated", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write without a delegation")
	}
}

func TestResetShell_NotYourDelegationRejected(t *testing.T) {
	svc, _, sessions := newTestService()
	seedDelegation(svc, "sess-1", "client-1", time.Now())

	if err := svc.ResetShell("client-2", "sess-1"); !errors.Is(err, ErrNotYourDelegation) {
		t.Fatalf("ResetShell() error = %v, want ErrNotYourDelegation", err)
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write for another client's delegation")
	}
}
