package resolve

import (
	"errors"
	"testing"

	"momo-shell/internal/adapter/out/shparse"
	"momo-shell/internal/core/port/out"
)

// fakeParser lets tests control exactly what Analyze returns without
// depending on the real shparse adapter (house style: hand-rolled fakes,
// no mock framework -- see shellintegration's fakeBuilder/fakeInjector).
type fakeParser struct {
	analysis out.CommandAnalysis
	err      error
}

func (f *fakeParser) Analyze(command, dialect string) (out.CommandAnalysis, error) {
	return f.analysis, f.err
}

var _ out.ShellParser = (*fakeParser)(nil)

func TestResolve_ParseErrorDegradesToHighUncertain(t *testing.T) {
	svc := New(Deps{Parser: &fakeParser{err: errors.New("boom")}})

	v := svc.Resolve("whatever", "bash")

	if v.Risk != RiskHigh {
		t.Errorf("Risk = %v, want %v", v.Risk, RiskHigh)
	}
	if !v.Uncertain {
		t.Error("expected Uncertain=true on parse failure")
	}
	if v.AutoRunnable() {
		t.Error("a parse failure must never be AutoRunnable")
	}
	if len(v.Reasons) == 0 {
		t.Error("expected at least one reason explaining the parse failure")
	}
}

func TestResolve_DelegatesToClassify(t *testing.T) {
	// A destructive command with a critical (literal "/") target, per the
	// risk matrix -- confirms Resolve actually routes through classify()
	// rather than, say, always returning a fixed Verdict.
	analysis := out.CommandAnalysis{
		Commands: []out.SimpleCommand{{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {Value: "/"}}}},
	}
	svc := New(Deps{Parser: &fakeParser{analysis: analysis}})

	v := svc.Resolve("rm -rf /", "bash")

	if v.Risk != RiskHigh {
		t.Errorf("Risk = %v, want %v", v.Risk, RiskHigh)
	}
	if v.AutoRunnable() {
		t.Error("rm -rf / must never be AutoRunnable")
	}
}

func TestResolve_SafeCommandIsAutoRunnable(t *testing.T) {
	analysis := out.CommandAnalysis{
		Commands: []out.SimpleCommand{{Verb: "ls", Args: []out.Arg{{Value: "-la"}}}},
	}
	svc := New(Deps{Parser: &fakeParser{analysis: analysis}})

	v := svc.Resolve("ls -la", "bash")

	if !v.AutoRunnable() {
		t.Errorf("expected ls -la to be AutoRunnable, got %+v", v)
	}
}

func TestVerdict_AutoRunnable(t *testing.T) {
	cases := []struct {
		name string
		v    Verdict
		want bool
	}{
		{"low, certain", Verdict{Risk: RiskLow, Uncertain: false}, true},
		{"low, uncertain", Verdict{Risk: RiskLow, Uncertain: true}, false},
		{"medium, certain", Verdict{Risk: RiskMedium, Uncertain: false}, false},
		{"high, certain", Verdict{Risk: RiskHigh, Uncertain: false}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.v.AutoRunnable(); got != tc.want {
				t.Errorf("AutoRunnable() = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- Integration smoke tests: the real shparse adapter, not a fake ---
// (plan's explicit ask: "adapter 실물 통합 스모크 1~2건") -- these are the
// only tests in this package that cross into internal/adapter, deliberately,
// to prove the two pieces work together end-to-end for the canonical
// acceptance cases (US-2/US-3).

func TestResolve_Integration_EmptyVarRmRequiresApproval(t *testing.T) {
	svc := New(Deps{Parser: shparse.New()})

	// The canonical US-3 example: whether $EMPTY is empty, unset, or
	// whitespace, this command must never auto-run.
	v := svc.Resolve("rm -rf /$EMPTY", "bash")

	if v.AutoRunnable() {
		t.Fatalf("rm -rf /$EMPTY must never be AutoRunnable, got %+v", v)
	}
	if v.Risk != RiskHigh {
		t.Errorf("Risk = %v, want %v", v.Risk, RiskHigh)
	}
}

func TestResolve_Integration_SafeCommandAutoRuns(t *testing.T) {
	svc := New(Deps{Parser: shparse.New()})

	v := svc.Resolve("git status", "bash")

	if !v.AutoRunnable() {
		t.Errorf("expected 'git status' to be AutoRunnable, got %+v", v)
	}
}

func TestResolve_Integration_UnsupportedDialectRequiresApproval(t *testing.T) {
	svc := New(Deps{Parser: shparse.New()})

	v := svc.Resolve("echo hi", "zsh")

	if v.AutoRunnable() {
		t.Errorf("an unsupported dialect must never be AutoRunnable, got %+v", v)
	}
}
