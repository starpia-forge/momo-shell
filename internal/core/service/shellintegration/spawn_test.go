package shellintegration

import (
	"bytes"
	"context"
	"testing"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// psShellPath is a realistic Windows PowerShell path -- resolveDialect
// matches on "powershell"/"pwsh" case-insensitively.
const psShellPath = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`

// --- PrepareSpawn / spawn-bootstrapped Attach ---

func TestPrepareSpawn_PowerShell_AttachEntersPhaseActiveImmediately(t *testing.T) {
	inj := &fakeInjector{shell: psShellPath, kind: domain.KindLocal, shellOK: true}
	b := &fakeBuilder{spawnDialect: out.DialectPowerShell}
	svc := New(Deps{Shell: inj, Builder: b})

	args, ok := svc.PrepareSpawn(testSessionID, psShellPath)
	if !ok {
		t.Fatal("expected PrepareSpawn to succeed for a PowerShell path")
	}
	if len(args) == 0 {
		t.Fatal("expected non-empty spawn args")
	}

	svc.Attach(testSessionID, domain.KindLocal)

	if inj.writeCount() != 0 {
		t.Fatalf("expected no WriteRaw for a spawn-bootstrapped session (nothing to type), got %d", inj.writeCount())
	}
	if got := svc.get(testSessionID).phase; got != phaseActive {
		t.Fatalf("expected phaseActive immediately after Attach, got %v", got)
	}

	// Ordinary text renders straight through (no suppression window at all).
	if got := svc.OnOutput(testSessionID, []byte("PS C:\\src> ")); string(got) != "PS C:\\src> " {
		t.Fatalf("expected immediate pass-through rendering, got %q", got)
	}
	if got := svc.OnOutput(testSessionID, []byte("\x1b]133;A\x07")); len(got) != 0 {
		t.Fatalf("expected the prompt marker to be stripped, got %q", got)
	}
	if !svc.AtPrompt(testSessionID) {
		t.Fatal("expected AtPrompt=true after the spawn-installed hook's first prompt marker")
	}
}

func TestPrepareSpawn_NonPowerShell_DeclinesWithoutCallingBuilder(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	b := &fakeBuilder{spawnDialect: out.DialectPowerShell} // only PowerShell opted in
	svc := New(Deps{Shell: inj, Builder: b})

	if args, ok := svc.PrepareSpawn(testSessionID, "/bin/bash"); ok {
		t.Fatalf("expected PrepareSpawn to decline a non-PowerShell path, got args=%v", args)
	}
	if b.spawnWasCalled() {
		t.Fatal("expected the dialect gate to short-circuit before the builder is ever called")
	}

	// Attach must fall back to the normal local readiness gate.
	svc.Attach(testSessionID, domain.KindLocal)
	if inj.writeCount() != 0 {
		t.Fatalf("expected the readiness gate (no immediate write), got %d", inj.writeCount())
	}
	svc.fireReady(testSessionID, svc.get(testSessionID))
	if inj.writeCount() != 1 {
		t.Fatalf("expected the readiness gate to fire the typed hook normally, got %d", inj.writeCount())
	}
}

func TestDiscardSpawn_ThenAttach_FallsBackToReadinessGate(t *testing.T) {
	inj := &fakeInjector{shell: psShellPath, kind: domain.KindLocal, shellOK: true}
	b := &fakeBuilder{spawnDialect: out.DialectPowerShell}
	svc := New(Deps{Shell: inj, Builder: b})

	if _, ok := svc.PrepareSpawn(testSessionID, psShellPath); !ok {
		t.Fatal("expected PrepareSpawn to succeed")
	}
	svc.DiscardSpawn(testSessionID)

	svc.Attach(testSessionID, domain.KindLocal)
	if inj.writeCount() != 0 {
		t.Fatalf("expected no immediate write right after Attach (gate armed, not fired), got %d", inj.writeCount())
	}
	svc.fireReady(testSessionID, svc.get(testSessionID))
	if inj.writeCount() != 1 {
		t.Fatalf("expected the typed-injection gate to fire after the spawn stash was discarded, got %d", inj.writeCount())
	}
	if !bytes.Contains(inj.lastWrite(), []byte("momostart_")) {
		t.Fatalf("expected the typed hook (with its sentinel) after falling back, got %q", inj.lastWrite())
	}
}

func TestPrepareSpawn_TakenOnce_StaleEntryDoesNotReactivateOnReattach(t *testing.T) {
	inj := &fakeInjector{shell: psShellPath, kind: domain.KindLocal, shellOK: true}
	b := &fakeBuilder{spawnDialect: out.DialectPowerShell}
	svc := New(Deps{Shell: inj, Builder: b})

	svc.PrepareSpawn(testSessionID, psShellPath)
	svc.Attach(testSessionID, domain.KindLocal) // takes and clears the pending entry
	if got := svc.get(testSessionID).phase; got != phaseActive {
		t.Fatalf("expected phaseActive on the first Attach, got %v", got)
	}
	svc.Detach(testSessionID)

	// Re-attach without a fresh PrepareSpawn -- must NOT find a stale entry.
	svc.Attach(testSessionID, domain.KindLocal)
	if got := svc.get(testSessionID).phase; got == phaseActive {
		t.Fatal("expected re-Attach without a fresh PrepareSpawn to NOT re-enter phaseActive")
	}
	if inj.writeCount() != 0 {
		t.Fatalf("expected the normal readiness gate (no immediate write) on re-attach, got %d", inj.writeCount())
	}
}

func TestPrepareSpawn_TwoSessions_DoNotCrossContaminate(t *testing.T) {
	inj := &fakeInjector{shell: psShellPath, kind: domain.KindLocal, shellOK: true}
	b := &fakeBuilder{spawnDialect: out.DialectPowerShell}
	svc := New(Deps{Shell: inj, Builder: b})

	const sessSpawn, sessGate = "sess-spawn", "sess-gate"
	svc.PrepareSpawn(sessSpawn, psShellPath)
	// sessGate never gets PrepareSpawn -- it must use the normal gate.

	svc.Attach(sessSpawn, domain.KindLocal)
	svc.Attach(sessGate, domain.KindLocal)

	if got := svc.get(sessSpawn).phase; got != phaseActive {
		t.Fatalf("expected sessSpawn to be phaseActive, got %v", got)
	}
	if got := svc.get(sessGate).phase; got == phaseActive {
		t.Fatal("expected sessGate to NOT be phaseActive (no PrepareSpawn was ever called for it)")
	}
	if inj.writeCount() != 0 {
		t.Fatalf("expected zero writes before sessGate's gate fires, got %d", inj.writeCount())
	}
	svc.fireReady(sessGate, svc.get(sessGate))
	if inj.writeCount() != 1 {
		t.Fatalf("expected exactly 1 write (sessGate's gate; sessSpawn never writes), got %d", inj.writeCount())
	}
}

func TestPrepareSpawn_QueryRoundTrip_DialectResolvesCorrectly(t *testing.T) {
	inj := &fakeInjector{shell: psShellPath, kind: domain.KindLocal, shellOK: true}
	b := &fakeBuilder{spawnDialect: out.DialectPowerShell}
	svc := New(Deps{Shell: inj, Builder: b})

	svc.PrepareSpawn(testSessionID, psShellPath)
	svc.Attach(testSessionID, domain.KindLocal)

	// st.dialect must come from the ordinary SessionShell -> resolveDialect
	// path (PrepareSpawn's take only sets nonce/phase), and must agree with
	// what PrepareSpawn resolved -- otherwise Query's ProbeScript call would
	// use the wrong dialect.
	if got := svc.get(testSessionID).dialect; got != out.DialectPowerShell {
		t.Fatalf("expected dialect=PowerShell after a spawn-bootstrapped Attach, got %q", got)
	}

	svc.OnOutput(testSessionID, []byte("\x1b]133;A\x07")) // the spawn hook's first prompt cycle
	if !svc.AtPrompt(testSessionID) {
		t.Fatal("expected AtPrompt=true after the first prompt marker")
	}

	want := VarValue{Set: true, Value: "hello"}
	inj.mu.Lock()
	inj.onWrite = func(script []byte) {
		nonce := probeNonceFromScript(script)
		reply := []byte("\x1b]1337;momo;probe;" + nonce + ";" + encodeProbeEntry("FOO", want) + "\x07")
		svc.OnOutput(testSessionID, reply)
	}
	inj.mu.Unlock()

	got, err := svc.Query(context.Background(), testSessionID, []string{"FOO"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if got["FOO"] != want {
		t.Fatalf("got %+v, want FOO=%+v", got, want)
	}
}

// --- Reinject guard for local PowerShell ---

func TestReinject_LocalPowerShell_Refused(t *testing.T) {
	inj := &fakeInjector{shell: psShellPath, kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	svc.Attach(testSessionID, domain.KindLocal) // typed gate path; dialect resolves to PowerShell via SessionShell

	if err := svc.Reinject(testSessionID); err == nil {
		t.Fatal("expected Reinject to refuse a local PowerShell session (would desync the terminal)")
	}
}
