package resolve

import (
	"strings"

	"momo-shell/internal/core/port/out"
)

// abortMarker is the sentinel the inline guard emits when it refuses to run
// a command whose variable-dependent target turned out empty or unset at
// execution time (doc 17 §6.3's TOCTOU backstop for L1's static verdict).
const abortMarker = "momo:abort"

// buildGuardedCommand rewrites command into a self-checking form when one is
// warranted (destructive verb + variable-dependent target), or returns ""
// when no rewrite is needed -- callers run the original command as-is in
// that case (doc 17 §6.1: "GuardedCmd ... 인라인 가드 삽입된 실제 실행 문자열").
//
// This is an if/then/else/fi compound, not the design doc's literal
// `... || { echo momo:abort; exit 1; }; <cmd>` example: that literal form
// needs `exit` to stop <cmd> from running unconditionally after the `;`,
// which would terminate the shared stateful shell session itself (doc 16
// §7-1's turn-based shared-shell model) on every guard trip. if/then/else
// blocks <cmd> without a subshell and without exiting, preserving state
// exactly as the design intends -- resolving the "안전한 문법" question
// doc 16:158 left open for implementation time.
func buildGuardedCommand(command string, a out.CommandAnalysis) string {
	if !needsGuard(a) {
		return ""
	}

	checks := make([]string, len(a.ReferencedVars))
	for i, name := range a.ReferencedVars {
		checks[i] = `[ -n "${` + name + `:-}" ]`
	}

	return "if " + strings.Join(checks, " && ") + "; then " + command + "; else echo " + abortMarker + "; fi"
}

// needsGuard reports whether a runtime self-check is warranted: a
// destructive command (risk.go's isDestructiveVerb) targeting a
// variable-dependent argument or redirect. Purely-literal destructive
// commands (e.g. "rm -rf /") have no variable to re-check at execution
// time, so there's nothing for the guard to defend against -- they rely on
// the approval gate alone.
func needsGuard(a out.CommandAnalysis) bool {
	if len(a.ReferencedVars) == 0 {
		return false
	}
	for _, cmd := range a.Commands {
		if isDestructiveVerb(cmd) && hasVarTarget(cmd) {
			return true
		}
	}
	return false
}

func hasVarTarget(cmd out.SimpleCommand) bool {
	for _, a := range cmd.Args {
		if a.HasVar {
			return true
		}
	}
	for _, r := range cmd.Redirects {
		if r.HasVar {
			return true
		}
	}
	return false
}
