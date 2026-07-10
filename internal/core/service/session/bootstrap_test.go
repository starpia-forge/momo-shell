package session

import (
	"testing"

	"momo-shell/internal/core/port/in"
)

func TestCreateLocal_ConsultsBootstrapperAndForwardsArgs(t *testing.T) {
	stream := newFakeStream()
	pub := &recordingPublisher{}
	opener := &fakeOpener{stream: stream}
	boot := &fakeBootstrapper{ok: true, args: []string{"-NoLogo", "-NoExit", "-EncodedCommand", "abc123"}}
	svc := New(Deps{LocalOpener: opener, Publisher: pub})
	svc.SetLocalBootstrapper(boot)
	defer svc.CloseAll()

	info, err := svc.CreateLocal(in.LocalOpts{Shell: "powershell.exe"})
	if err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}

	prepared, shells, discards := boot.snapshot()
	if len(prepared) != 1 || prepared[0] != info.ID {
		t.Fatalf("expected PrepareSpawn called once with the session's own ID, got %v (session ID %s)", prepared, info.ID)
	}
	if len(shells) != 1 || shells[0] != "powershell.exe" {
		t.Fatalf("expected PrepareSpawn called with the resolved shell path, got %v", shells)
	}
	if len(discards) != 0 {
		t.Fatalf("expected no DiscardSpawn on a successful Open, got %v", discards)
	}

	if got := opener.getLastArgs(); len(got) != 4 || got[3] != "abc123" {
		t.Fatalf("expected PrepareSpawn's args to reach Open, got %v", got)
	}
}

func TestCreateLocal_BootstrapperDeclines_OpenGetsNoExtraArgs(t *testing.T) {
	stream := newFakeStream()
	pub := &recordingPublisher{}
	opener := &fakeOpener{stream: stream}
	boot := &fakeBootstrapper{ok: false} // e.g. non-PowerShell dialect
	svc := New(Deps{LocalOpener: opener, Publisher: pub})
	svc.SetLocalBootstrapper(boot)
	defer svc.CloseAll()

	if _, err := svc.CreateLocal(in.LocalOpts{Shell: "/bin/bash"}); err != nil {
		t.Fatalf("CreateLocal failed: %v", err)
	}
	if got := opener.getLastArgs(); len(got) != 0 {
		t.Fatalf("expected no extra args when the bootstrapper declines, got %v", got)
	}
}

func TestCreateLocal_OpenFails_DiscardsSpawn(t *testing.T) {
	pub := &recordingPublisher{}
	opener := &fakeOpener{err: errBoom}
	boot := &fakeBootstrapper{ok: true, args: []string{"-EncodedCommand", "abc123"}}
	svc := New(Deps{LocalOpener: opener, Publisher: pub})
	svc.SetLocalBootstrapper(boot)

	if _, err := svc.CreateLocal(in.LocalOpts{Shell: "powershell.exe"}); err == nil {
		t.Fatal("expected an error when the opener fails")
	}

	_, _, discards := boot.snapshot()
	if len(discards) != 1 {
		t.Fatalf("expected DiscardSpawn called once after Open failed, got %v", discards)
	}
}

func TestCreateLocal_NilBootstrapper_IsHarmless(t *testing.T) {
	stream := newFakeStream()
	pub := &recordingPublisher{}
	svc := New(Deps{LocalOpener: &fakeOpener{stream: stream}, Publisher: pub})
	defer svc.CloseAll()

	if _, err := svc.CreateLocal(in.LocalOpts{Shell: "bash"}); err != nil {
		t.Fatalf("expected CreateLocal to work with no bootstrapper registered, got %v", err)
	}
}
