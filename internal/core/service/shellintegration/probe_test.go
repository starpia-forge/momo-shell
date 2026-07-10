package shellintegration

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

// bootstrapToPrompt drives svc through Attach + the full bootstrap sequence
// (sentinel -> hookinstalled -> a real prompt_start) so testSessionID ends
// up phaseActive with AtPrompt()==true -- the precondition Query requires.
// Mirrors TestBootstrap_FullLifecycle's manual sequence. Uses KindSSH so
// injection happens synchronously (Query's own gating logic, not the
// readiness gate, is what these tests exercise -- see TestReadinessGate_*
// in middleware_test.go for the local-injection-deferral behavior).
func bootstrapToPrompt(t *testing.T, svc *Service, inj *fakeInjector) {
	t.Helper()
	svc.Attach(testSessionID, domain.KindSSH)
	nonce := extractNonce(inj.lastWrite())
	if nonce == "" {
		t.Fatal("failed to extract nonce from injected hook script")
	}
	svc.OnOutput(testSessionID, []byte("momostart_"+nonce+"\n"))
	svc.OnOutput(testSessionID, []byte("\x1b]1337;momo;hookinstalled;"+nonce+"\x07"))
	svc.OnOutput(testSessionID, []byte("\x1b]133;A\x07"))
	if !svc.AtPrompt(testSessionID) {
		t.Fatal("expected AtPrompt=true after bootstrap")
	}
}

// probeNonceFromScript extracts the nonce fakeBuilder.ProbeScript embedded
// ("PROBE:<nonce>:<vars>") so a test's onWrite hook can address its reply
// to the right in-flight Query call.
func probeNonceFromScript(script []byte) string {
	parts := strings.SplitN(string(script), ":", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func encodeProbeEntry(name string, v VarValue) string {
	if !v.Set {
		return name + "=0:;"
	}
	return name + "=1:" + base64.StdEncoding.EncodeToString([]byte(v.Value)) + ";"
}

func TestQuery_RoundTrip_DecodesFourStates(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	bootstrapToPrompt(t, svc, inj)

	want := map[string]VarValue{
		"FOO":   {Set: true, Value: "hello"},
		"EMPTY": {Set: true, Value: ""},
		"SPACE": {Set: true, Value: "   "},
		"BAZ":   {Set: false},
	}
	inj.mu.Lock()
	inj.onWrite = func(script []byte) {
		nonce := probeNonceFromScript(script)
		var payload strings.Builder
		for _, name := range []string{"FOO", "EMPTY", "SPACE", "BAZ"} {
			payload.WriteString(encodeProbeEntry(name, want[name]))
		}
		reply := []byte("\x1b]1337;momo;probe;" + nonce + ";" + payload.String() + "\x07")
		svc.OnOutput(testSessionID, reply)
	}
	inj.mu.Unlock()

	got, err := svc.Query(context.Background(), testSessionID, []string{"FOO", "EMPTY", "SPACE", "BAZ"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for name, wantVal := range want {
		if got[name] != wantVal {
			t.Errorf("%s = %+v, want %+v", name, got[name], wantVal)
		}
	}

	if inj.writeCount() != 2 {
		t.Fatalf("expected 2 WriteRaw calls (hook install + probe), got %d", inj.writeCount())
	}
}

func TestQuery_NotAtPrompt_ReturnsErrorWithoutInjecting(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	svc.Attach(testSessionID, domain.KindSSH) // bootstrap not completed -- not phaseActive yet

	_, err := svc.Query(context.Background(), testSessionID, []string{"FOO"})
	if !errors.Is(err, ErrNotAtPrompt) {
		t.Fatalf("expected ErrNotAtPrompt, got %v", err)
	}
	if inj.writeCount() != 1 {
		t.Fatalf("expected no additional WriteRaw beyond the hook script, got %d", inj.writeCount())
	}
}

func TestQuery_UnattachedSession_ReturnsError(t *testing.T) {
	svc := New(Deps{Shell: &fakeInjector{}, Builder: &fakeBuilder{}})
	_, err := svc.Query(context.Background(), "never-attached", []string{"FOO"})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestQuery_InvalidVarName_ReturnsErrorWithoutInjecting(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	bootstrapToPrompt(t, svc, inj)
	writesBefore := inj.writeCount()

	for _, bad := range []string{"", "1BAD", "FOO;rm -rf /", "FOO BAR", "FOO$(id)"} {
		if _, err := svc.Query(context.Background(), testSessionID, []string{bad}); err == nil {
			t.Errorf("expected an error for invalid variable name %q, got nil", bad)
		}
	}
	if got := inj.writeCount(); got != writesBefore {
		t.Fatalf("expected no injection attempt for invalid names, write count changed %d -> %d", writesBefore, got)
	}
}

func TestQuery_TimeoutWhenNoReplyArrives(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	bootstrapToPrompt(t, svc, inj)
	// No onWrite hook -- the probe script is injected but nothing ever
	// replies (e.g. WriteRaw silently failed to reach a real shell, or the
	// reply was truncated past recognition -- doc 19 §4.1).

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := svc.Query(ctx, testSessionID, []string{"FOO"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestQuery_ConcurrentProbesDoNotCrossTalk(t *testing.T) {
	inj := &fakeInjector{shell: "/bin/bash", kind: domain.KindLocal, shellOK: true}
	svc := New(Deps{Shell: inj, Builder: &fakeBuilder{}})
	bootstrapToPrompt(t, svc, inj)

	// A single, never-reassigned hook that replies based on which variable
	// name is actually in the script it was handed -- unlike reassigning
	// inj.onWrite per goroutine (racy: goroutine B's assignment can land
	// before goroutine A's own WriteRaw reads the hook), this has no shared
	// mutable state for the two concurrent Query calls to race over.
	replies := map[string]VarValue{
		"ALPHA": {Set: true, Value: "one"},
		"BETA":  {Set: false},
	}
	inj.mu.Lock()
	inj.onWrite = func(script []byte) {
		parts := strings.SplitN(string(script), ":", 3)
		if len(parts) < 3 {
			return
		}
		nonce, vars := parts[1], strings.Split(parts[2], ",")
		var payload strings.Builder
		for _, name := range vars {
			payload.WriteString(encodeProbeEntry(name, replies[name]))
		}
		svc.OnOutput(testSessionID, []byte("\x1b]1337;momo;probe;"+nonce+";"+payload.String()+"\x07"))
	}
	inj.mu.Unlock()

	var wg sync.WaitGroup
	results := make([]map[string]VarValue, 2)
	errs := make([]error, 2)
	names := [][]string{{"ALPHA"}, {"BETA"}}

	wg.Add(2)
	for i := range names {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = svc.Query(context.Background(), testSessionID, names[i])
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("query %d failed: %v", i, err)
		}
	}
	// Each Query call's result must contain exactly the variable IT asked
	// for -- if nonces crossed, one side would see the other's variable
	// name instead (not just a wrong value).
	if v, ok := results[0]["ALPHA"]; !ok || v != (VarValue{Set: true, Value: "one"}) {
		t.Errorf("query 0 (ALPHA) = %+v, want {ALPHA:{Set:true Value:one}}", results[0])
	}
	if _, ok := results[0]["BETA"]; ok {
		t.Errorf("query 0 must not see BETA at all, got %+v", results[0])
	}
	if v, ok := results[1]["BETA"]; !ok || v != (VarValue{Set: false}) {
		t.Errorf("query 1 (BETA) = %+v, want {BETA:{Set:false}}", results[1])
	}
	if _, ok := results[1]["ALPHA"]; ok {
		t.Errorf("query 1 must not see ALPHA at all, got %+v", results[1])
	}
}

func TestDecodeProbePayload_SkipsMalformedEntries(t *testing.T) {
	got := decodeProbePayload("GOOD=1:aGVsbG8=;NOEQUALS;NOCOLON=1;BADFLAG=9:xx;OK=0:;")
	want := map[string]VarValue{
		"GOOD": {Set: true, Value: "hello"},
		"OK":   {Set: false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for name, v := range want {
		if got[name] != v {
			t.Errorf("%s = %+v, want %+v", name, got[name], v)
		}
	}
}

func TestValidVarName(t *testing.T) {
	valid := []string{"FOO", "_foo", "foo_bar2", "A"}
	invalid := []string{"", "1FOO", "FOO BAR", "FOO;BAR", "FOO$(x)", "FOO-BAR"}
	for _, n := range valid {
		if !validVarName(n) {
			t.Errorf("validVarName(%q) = false, want true", n)
		}
	}
	for _, n := range invalid {
		if validVarName(n) {
			t.Errorf("validVarName(%q) = true, want false", n)
		}
	}
}
