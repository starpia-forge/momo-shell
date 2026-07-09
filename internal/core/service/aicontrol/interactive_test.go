package aicontrol

import (
	"errors"
	"testing"
	"time"

	"momo-shell/internal/adapter/out/shparse"
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
	"momo-shell/internal/core/service/aicontrol/resolve"
)

func cmd(verb string, args ...string) out.SimpleCommand {
	sc := out.SimpleCommand{Verb: verb}
	for _, a := range args {
		sc.Args = append(sc.Args, out.Arg{Value: a})
	}
	return sc
}

func TestDetectInteractive(t *testing.T) {
	cases := []struct {
		name     string
		commands []out.SimpleCommand
		wantVerb string
		wantHit  bool
	}{
		{"vim with a file arg", []out.SimpleCommand{cmd("vim", "notes.txt")}, "vim", true},
		{"vi bare", []out.SimpleCommand{cmd("vi")}, "vi", true},
		{"nvim bare", []out.SimpleCommand{cmd("nvim")}, "nvim", true},
		{"top bare", []out.SimpleCommand{cmd("top")}, "top", true},
		{"htop bare", []out.SimpleCommand{cmd("htop")}, "htop", true},
		{"less a file", []out.SimpleCommand{cmd("less", "app.log")}, "less", true},
		{"more a file", []out.SimpleCommand{cmd("more", "app.log")}, "more", true},
		{"python bare REPL", []out.SimpleCommand{cmd("python")}, "python", true},
		{"python3 bare REPL", []out.SimpleCommand{cmd("python3")}, "python3", true},
		{"python -c is not interactive", []out.SimpleCommand{cmd("python", "-c", "print(1)")}, "", false},
		{"python with a script arg is not interactive", []out.SimpleCommand{cmd("python", "a.py")}, "", false},
		{"python -m module is not interactive", []out.SimpleCommand{cmd("python", "-m", "http.server")}, "", false},
		{"apt install without -y prompts", []out.SimpleCommand{cmd("apt", "install", "foo")}, "apt", true},
		{"apt-get remove without -y prompts", []out.SimpleCommand{cmd("apt-get", "remove", "foo")}, "apt-get", true},
		{"apt install -y does not prompt", []out.SimpleCommand{cmd("apt", "install", "-y", "foo")}, "", false},
		{"apt install --yes does not prompt", []out.SimpleCommand{cmd("apt", "install", "--yes", "foo")}, "", false},
		{"apt list is read-only", []out.SimpleCommand{cmd("apt", "list")}, "", false},
		{"plain ls is not interactive", []out.SimpleCommand{cmd("ls", "-la")}, "", false},
		{"plain cat is not interactive", []out.SimpleCommand{cmd("cat", "f")}, "", false},
		{"pipeline matches the interactive stage", []out.SimpleCommand{cmd("journalctl", "-u", "x"), cmd("less")}, "less", true},
		{"empty analysis (parse failure) has no match", nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hint, ok := detectInteractive(out.CommandAnalysis{Commands: tc.commands})
			if ok != tc.wantHit {
				t.Fatalf("detectInteractive() ok = %v, want %v (hint=%+v)", ok, tc.wantHit, hint)
			}
			if ok && hint.Verb != tc.wantVerb {
				t.Errorf("hint.Verb = %q, want %q", hint.Verb, tc.wantVerb)
			}
			if ok && hint.Suggestion == "" {
				t.Error("expected a non-empty Suggestion when a hint is returned")
			}
		})
	}
}

func TestRunCommand_InteractiveCommandIsRejectedBeforeInjection(t *testing.T) {
	verdict := resolve.Verdict{
		Risk:     resolve.RiskLow,
		Analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{cmd("vim", "notes.txt")}},
	}
	svc, pub, sessions := newTestServiceWithResolver(verdict)
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now().Add(-time.Hour))

	_, err := svc.RunCommand("client-1", "sess-1", "vim notes.txt")

	var interactiveErr *InteractiveCommandError
	if !errors.As(err, &interactiveErr) {
		t.Fatalf("RunCommand() error = %v, want *InteractiveCommandError", err)
	}
	if interactiveErr.Verb != "vim" {
		t.Errorf("interactiveErr.Verb = %q, want %q", interactiveErr.Verb, "vim")
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write to the session for a rejected interactive command")
	}
	if len(pub.all()) != 0 {
		t.Error("expected no mcp:cmd-approval or mcp:cmd-state event for a rejected interactive command")
	}
}

func TestRunCommand_NonInteractiveCommandIsUnaffected(t *testing.T) {
	verdict := resolve.Verdict{
		Risk:     resolve.RiskLow,
		Analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{cmd("ls", "-la")}},
	}
	svc, _, sessions := newTestServiceWithResolver(verdict)
	sessions.sessionShellFunc = existingSession("sess-1")
	seedDelegation(svc, "sess-1", "client-1", time.Now().Add(-time.Hour))

	handle, err := svc.RunCommand("client-1", "sess-1", "ls -la")

	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	if handle.State != domain.CmdRunning {
		t.Errorf("handle.State = %v, want %v", handle.State, domain.CmdRunning)
	}
	if len(sessions.allWrites()) != 1 {
		t.Errorf("expected one write, got %d", len(sessions.allWrites()))
	}
}

// --- Integration smoke test: the real shparse + resolve pipeline, not a
// fake resolver -- proves the seed ruleset matches what the real parser
// actually produces for a canonical case (design doc 16 §141's "top").

func TestRunCommand_Integration_TopIsRejectedBeforeInjection(t *testing.T) {
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

	_, err := svc.RunCommand("client-1", "sess-1", "top")

	var interactiveErr *InteractiveCommandError
	if !errors.As(err, &interactiveErr) {
		t.Fatalf("RunCommand() error = %v, want *InteractiveCommandError", err)
	}
	if interactiveErr.Verb != "top" {
		t.Errorf("interactiveErr.Verb = %q, want %q", interactiveErr.Verb, "top")
	}
	if len(sessions.allWrites()) != 0 {
		t.Error("expected no write to the session for a rejected interactive command")
	}
}
