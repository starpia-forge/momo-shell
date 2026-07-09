// Package resolve implements static command-safety analysis (design doc 17
// §6.1's L1 gate, FR-5 ①): given a shell command, classify whether it's
// safe to auto-run or must be gated behind human approval. This is
// static-only -- no live shell values (doc 18 §0.1 splits the design's L1
// pipeline into C1, here, which degrades any referenced variable straight
// to approval, and C2, shellintegration.Service.Query, which will later let
// a consumer upgrade that judgment with live values).
package resolve

import (
	"strings"

	"momo-shell/internal/core/port/out"
)

// RiskLevel is a Verdict's risk classification (design doc 17 §6.1: "low /
// medium / high" -- undefined as code there, defined here).
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Verdict is a command's static safety classification. Fields the design
// (doc 17 §6.1) assigns to later work items (ResolvedCmd/GuardedCmd to C2/
// C3's live-value + inline-guard rewrite, ContextHop to D5's nested-shell
// handling) are intentionally omitted here and added when that work makes
// them meaningful, rather than speculatively stubbed now.
type Verdict struct {
	Risk      RiskLevel
	Uncertain bool
	// GuardedCmd is the inline-guard-rewritten form of the command, meant to
	// run in place of the original once approved (doc 17 §6.3's runtime
	// TOCTOU backstop, layered on top of -- not instead of -- L1's static
	// verdict above). Empty means no rewrite was needed: run the original
	// command as-is (safe command, no variable-dependent destructive
	// target, or static analysis failed).
	GuardedCmd string
	Reasons    []string
	// Analysis is the parsed command this Verdict was classified from --
	// zero-valued (empty Commands) when static analysis failed (unsupported
	// dialect, parse error). Exposed for D4a's interactive-command detector
	// (aicontrol.detectInteractive), which needs the same parsed verb/args
	// data classify already consumed, rather than re-parsing.
	Analysis out.CommandAnalysis
}

// AutoRunnable reports whether a command may execute without approval --
// FR-5's default rule: low risk AND no uncertainty.
func (v Verdict) AutoRunnable() bool {
	return v.Risk == RiskLow && !v.Uncertain
}

// Deps are the out-ports/collaborators Service needs.
type Deps struct {
	Parser out.ShellParser
}

// Service implements static command-safety resolution.
type Service struct {
	parser out.ShellParser
}

func New(deps Deps) *Service {
	return &Service{parser: deps.Parser}
}

// Resolve classifies command under dialect. It never returns an error --
// every failure mode (unsupported dialect, parse failure) is expressed as
// a conservative Verdict instead, so a caller can't accidentally treat "an
// error occurred" as "safe to proceed" by mishandling an err return (doc
// 18 risk register: "실패 시 참조변수=불확실→승인").
func (s *Service) Resolve(command, dialect string) Verdict {
	analysis, err := s.parser.Analyze(command, dialect)
	if err != nil {
		return Verdict{
			Risk:      RiskHigh,
			Uncertain: true,
			Reasons:   []string{"static analysis failed: " + err.Error()},
		}
	}
	v := classify(analysis)
	v.GuardedCmd = buildGuardedCommand(strings.TrimSpace(command), analysis)
	v.Analysis = analysis
	return v
}
