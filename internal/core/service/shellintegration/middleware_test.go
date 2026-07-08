package shellintegration

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// --- hand-rolled fakes (house style: internal/core/service/session's own
// tests use recorder structs + mutex, not testify) ---

type fakeInjector struct {
	mu       sync.Mutex
	writes   [][]byte
	shell    string
	kind     domain.SessionKind
	shellOK  bool
	writeErr error

	// onWrite, if set, is called synchronously with each write's bytes
	// after recording it -- used by probe tests to synthesize a reply
	// in-line before WriteRaw returns to its caller (Query), avoiding any
	// goroutine-timing dependency in the test.
	onWrite func(data []byte)
}

func (f *fakeInjector) WriteRaw(sessionID string, data []byte) error {
	f.mu.Lock()
	f.writes = append(f.writes, append([]byte{}, data...))
	err := f.writeErr
	hook := f.onWrite
	f.mu.Unlock()
	if hook != nil {
		hook(data)
	}
	return err
}

func (f *fakeInjector) SessionShell(sessionID string) (string, domain.SessionKind, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.shell, f.kind, f.shellOK
}

func (f *fakeInjector) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.writes)
}

func (f *fakeInjector) lastWrite() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.writes) == 0 {
		return nil
	}
	return f.writes[len(f.writes)-1]
}

// fakeBuilder emits a minimal script with the same shape (momostart
// sentinel ... hookinstalled marker) as the real bash/powershell
// templates, without depending on the adapter package.
type fakeBuilder struct {
	unsupported out.ShellDialect
}

func (b *fakeBuilder) HookScript(dialect out.ShellDialect, nonce string) ([]byte, error) {
	if b.unsupported != "" && dialect == b.unsupported {
		return nil, errors.New("fakeBuilder: unsupported dialect")
	}
	return []byte("momostart_" + nonce + "\nNOISE\n\x1b]1337;momo;hookinstalled;" + nonce + "\x07"), nil
}

func (b *fakeBuilder) ProbeScript(dialect out.ShellDialect, nonce string, vars []string) ([]byte, error) {
	if b.unsupported != "" && dialect == b.unsupported {
		return nil, errors.New("fakeBuilder: unsupported dialect")
	}
	return []byte("PROBE:" + nonce + ":" + strings.Join(vars, ",")), nil
}

func extractNonce(script []byte) string {
	const prefix = "momostart_"
	idx := bytes.Index(script, []byte(prefix))
	if idx < 0 {
		return ""
	}
	rest := script[idx+len(prefix):]
	if end := bytes.IndexByte(rest, '\n'); end >= 0 {
		return string(rest[:end])
	}
	return string(rest)
}

type recordingObserver struct {
	mu            sync.Mutex
	prompts       []string
	commandStarts []string
	commandEnds   []commandEndRecord
	altScreens    []altScreenRecord
}

type commandEndRecord struct {
	sessionID string
	exitCode  int
}

type altScreenRecord struct {
	sessionID string
	entered   bool
}

func (r *recordingObserver) OnPrompt(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prompts = append(r.prompts, id)
}

func (r *recordingObserver) OnCommandStart(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commandStarts = append(r.commandStarts, id)
}

func (r *recordingObserver) OnCommandEnd(id string, exitCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commandEnds = append(r.commandEnds, commandEndRecord{id, exitCode})
}

func (r *recordingObserver) OnAltScreen(id string, entered bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.altScreens = append(r.altScreens, altScreenRecord{id, entered})
}

func (r *recordingObserver) snapshot() recordingObserver {
	r.mu.Lock()
	defer r.mu.Unlock()
	return recordingObserver{
		prompts:       append([]string{}, r.prompts...),
		commandStarts: append([]string{}, r.commandStarts...),
		commandEnds:   append([]commandEndRecord{}, r.commandEnds...),
		altScreens:    append([]altScreenRecord{}, r.altScreens...),
	}
}

const testSessionID = "sess-1"

// --- Attach: dialect resolution ---

func TestAttach_KnownLocalDialect_InjectsHookScript(t *testing.T) {
	inj := &fakeInjector{shell: `C:\Program Files\Git\bin\bash.exe`, kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})

	svc.Attach(testSessionID, domain.KindLocal)

	if inj.writeCount() != 1 {
		t.Fatalf("expected exactly 1 WriteRaw call, got %d", inj.writeCount())
	}
	if !bytes.Contains(inj.lastWrite(), []byte("momostart_")) {
		t.Fatalf("expected the injected script to contain the momostart sentinel, got %q", inj.lastWrite())
	}
}

func TestAttach_UnknownLocalDialect_NoInjection(t *testing.T) {
	inj := &fakeInjector{shell: "cmd.exe", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})

	svc.Attach(testSessionID, domain.KindLocal)

	if inj.writeCount() != 0 {
		t.Fatalf("expected no injection for an unrecognized shell, got %d WriteRaw calls", inj.writeCount())
	}
	// Pass-through contract: OnOutput must be a pure no-op for this session.
	out := svc.OnOutput(testSessionID, []byte("anything\x1b]133;A\x07"))
	if string(out) != "anything\x1b]133;A\x07" {
		t.Fatalf("expected untouched pass-through, got %q", out)
	}
}

func TestAttach_SSH_AssumesBashRegardlessOfSessionShell(t *testing.T) {
	// SessionShell would report something else entirely (or ok=false) --
	// SSH must still assume bash per the design decision (doc: remote shell
	// is never recorded).
	inj := &fakeInjector{shellOK: false}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})

	svc.Attach(testSessionID, domain.KindSSH)

	if inj.writeCount() != 1 {
		t.Fatalf("expected SSH to trigger injection (bash assumed), got %d WriteRaw calls", inj.writeCount())
	}
}

func TestAttach_BuilderError_DegradesSafely(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{unsupported: out.DialectBash}})

	svc.Attach(testSessionID, domain.KindLocal)

	if inj.writeCount() != 0 {
		t.Fatalf("expected no WriteRaw when the builder errors, got %d", inj.writeCount())
	}
	out := svc.OnOutput(testSessionID, []byte("x"))
	if string(out) != "x" {
		t.Fatalf("expected pass-through after a builder error, got %q", out)
	}
}

func TestOnOutput_UnattachedSession_PassesThrough(t *testing.T) {
	svc := New(Deps{Shell: &fakeInjector{}, Builder: &fakeBuilder{}})
	got := svc.OnOutput("never-attached", []byte("raw bytes"))
	if string(got) != "raw bytes" {
		t.Fatalf("expected untouched pass-through for an unknown session, got %q", got)
	}
}

// --- Bootstrap: MOTD preserved, injection noise suppressed, then active ---

func TestBootstrap_FullLifecycle(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	obs := &recordingObserver{}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	svc.AddObserver(obs)

	svc.Attach(testSessionID, domain.KindLocal)
	nonce := extractNonce(inj.lastWrite())
	if nonce == "" {
		t.Fatal("failed to extract nonce from injected script")
	}

	// 1. Content arriving before our sentinel (e.g. an SSH MOTD) is buffered,
	// not lost -- released once the sentinel-containing chunk arrives (see
	// stepLocked's phaseWaitingSentinel comment for why this doesn't
	// release incrementally).
	motd := []byte("Welcome to Ubuntu 20.04\n")
	if got := svc.OnOutput(testSessionID, motd); len(got) != 0 {
		t.Fatalf("expected MOTD to be buffered pending the sentinel, got %q", got)
	}
	if svc.AtPrompt(testSessionID) {
		t.Fatal("AtPrompt should be false before hooks are confirmed installed")
	}

	// 2. The terminal echoes our own injected line back (sentinel onward).
	// Everything accumulated before the sentinel (the buffered MOTD) is
	// released now; the sentinel onward is suppressed (rendered = empty).
	sentinelOnward := []byte("momostart_" + nonce + "\nNOISE\n")
	got := svc.OnOutput(testSessionID, sentinelOnward)
	if !bytes.Equal(got, motd) {
		t.Fatalf("expected the buffered MOTD to be released once the sentinel arrived, got %q want %q", got, motd)
	}

	hookInstalled := []byte("\x1b]1337;momo;hookinstalled;" + nonce + "\x07")
	if got := svc.OnOutput(testSessionID, hookInstalled); len(got) != 0 {
		t.Fatalf("expected the hookinstalled marker itself to be stripped, got %q", got)
	}

	// 3. Now active: a real OSC133 prompt cycle must render cleanly (marker
	// stripped) and notify the observer.
	if got := svc.OnOutput(testSessionID, []byte("\x1b]133;A\x07")); len(got) != 0 {
		t.Fatalf("expected the prompt marker to be stripped from rendered output, got %q", got)
	}
	if !svc.AtPrompt(testSessionID) {
		t.Fatal("expected AtPrompt=true after a real prompt_start event")
	}

	cmdCycle := []byte("\x1b]133;C\x07echo hello\r\nhello\r\n\x1b]133;D;0\x07\x1b]133;A\x07")
	rendered := svc.OnOutput(testSessionID, cmdCycle)
	if want := "echo hello\r\nhello\r\n"; string(rendered) != want {
		t.Fatalf("rendered = %q, want %q (OSC markers stripped, real output kept)", rendered, want)
	}
	if !svc.AtPrompt(testSessionID) {
		t.Fatal("expected AtPrompt=true after the command's own prompt_start")
	}

	snap := obs.snapshot()
	if len(snap.prompts) != 2 {
		t.Fatalf("expected 2 OnPrompt calls, got %d: %+v", len(snap.prompts), snap.prompts)
	}
	if len(snap.commandStarts) != 1 {
		t.Fatalf("expected 1 OnCommandStart call, got %d", len(snap.commandStarts))
	}
	if len(snap.commandEnds) != 1 || snap.commandEnds[0].exitCode != 0 {
		t.Fatalf("expected 1 OnCommandEnd(exitCode=0), got %+v", snap.commandEnds)
	}
}

func TestBootstrap_GivesUpAfterMaxSentinelWait(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	svc.Attach(testSessionID, domain.KindLocal)

	// Feed output that never contains our sentinel (e.g. WriteRaw silently
	// failed to reach a real shell) past the give-up threshold.
	noise := []byte(strings.Repeat("x", maxSentinelWait+100))
	got := svc.OnOutput(testSessionID, noise)
	if !bytes.Equal(got, noise) {
		t.Fatalf("expected the buffered noise to flush once the cap is exceeded, got %d bytes want %d", len(got), len(noise))
	}

	// Degraded to pass-through: further output (even something sentinel-
	// shaped) must not be touched or suppressed.
	next := []byte("momostart_should-not-matter-anymore")
	if got := svc.OnOutput(testSessionID, next); !bytes.Equal(got, next) {
		t.Fatalf("expected pass-through after giving up, got %q", got)
	}
}

// --- Reinject ---

func TestReinject_ResetsStateAndSendsNewNonce(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	svc.Attach(testSessionID, domain.KindLocal)
	firstNonce := extractNonce(inj.lastWrite())

	if err := svc.Reinject(testSessionID); err != nil {
		t.Fatalf("Reinject failed: %v", err)
	}
	if inj.writeCount() != 2 {
		t.Fatalf("expected a second WriteRaw call from Reinject, got %d", inj.writeCount())
	}
	secondNonce := extractNonce(inj.lastWrite())
	if secondNonce == "" || secondNonce == firstNonce {
		t.Fatalf("expected a fresh nonce on reinject, got first=%q second=%q", firstNonce, secondNonce)
	}

	// State must reset -- feeding the OLD hookinstalled marker must NOT
	// reactivate (nonce no longer matches).
	stale := []byte("\x1b]1337;momo;hookinstalled;" + firstNonce + "\x07")
	svc.OnOutput(testSessionID, stale)
	if svc.AtPrompt(testSessionID) {
		t.Fatal("a stale nonce's hookinstalled marker must not reactivate the session")
	}
}

func TestReinject_UnattachedSession(t *testing.T) {
	svc := New(Deps{Shell: &fakeInjector{}, Builder: &fakeBuilder{}})
	if err := svc.Reinject("never-attached"); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

// --- Alt-screen ---

func TestAltScreen_ObserverFiresOnceOnEnterAndExit(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	obs := &recordingObserver{}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	svc.AddObserver(obs)
	svc.Attach(testSessionID, domain.KindLocal)
	nonce := extractNonce(inj.lastWrite())
	svc.OnOutput(testSessionID, []byte("momostart_"+nonce+"\n"))
	svc.OnOutput(testSessionID, []byte("\x1b]1337;momo;hookinstalled;"+nonce+"\x07"))

	svc.OnOutput(testSessionID, []byte("before\x1b[?1049hvim screen"))
	svc.OnOutput(testSessionID, []byte("still in vim"))
	svc.OnOutput(testSessionID, []byte("\x1b[?1049lback to normal"))

	snap := obs.snapshot()
	if len(snap.altScreens) != 2 {
		t.Fatalf("expected exactly 2 OnAltScreen calls (one enter, one exit), got %d: %+v", len(snap.altScreens), snap.altScreens)
	}
	if !snap.altScreens[0].entered {
		t.Errorf("expected first OnAltScreen call to be entered=true, got %+v", snap.altScreens[0])
	}
	if snap.altScreens[1].entered {
		t.Errorf("expected second OnAltScreen call to be entered=false, got %+v", snap.altScreens[1])
	}
}
