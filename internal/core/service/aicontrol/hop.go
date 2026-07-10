package aicontrol

import (
	"fmt"
	"strings"

	"momo-shell/internal/core/port/out"
)

// hopHint is D5a's per-command steering result: a matched context-hop verb
// (one that leaves the delegated session's custody by entering a new
// execution context -- remote host, another user, a container) and the
// human-facing reason to surface in the approval dialog (design doc 26 §1,
// doc 25 §6(b)'s low-cost interim mitigation). Static-only, mirroring D4a's
// interactiveHint -- see resolve.Verdict.Analysis, the parsed a.Commands
// this operates on. Unlike D4a, a hop match doesn't reject the command; it
// demotes it to human approval (see RunCommand's demoted flag).
type hopHint struct {
	Verb   string
	Reason string
}

// hopVerbs always hop custody regardless of arguments.
var hopVerbs = map[string]string{
	"ssh": "ssh leaves the delegated session's custody for a new remote context",
	"su":  "su leaves the delegated session's custody for a new user context",
}

// sudoInteractiveFlags are the sudo flags that open an interactive shell in
// a new custody context. Plain "sudo <cmd>" (one-shot privilege escalation
// that runs to completion in the same session) is not a hop -- that's the
// risk matrix's concern, not D5a's.
var sudoInteractiveFlags = map[string]bool{"-i": true, "-s": true}

// containerExecVerbs hop custody only when their first positional argument
// is "exec" (e.g. "docker exec" enters a running container; "docker ps"
// doesn't).
var containerExecVerbs = map[string]bool{"docker": true, "kubectl": true}

// detectHop scans a parsed command's pipeline stages, in order, for the
// first verb that would leave the delegated session's custody (design doc
// 26 §1). a.Commands is empty when static analysis couldn't parse the
// command -- that already degrades to the approval gate via
// Verdict.Uncertain, so this reports no match rather than guessing.
func detectHop(a out.CommandAnalysis) (hopHint, bool) {
	for _, cmd := range a.Commands {
		if hint, ok := classifyHop(cmd); ok {
			return hint, true
		}
	}
	return hopHint{}, false
}

func classifyHop(cmd out.SimpleCommand) (hopHint, bool) {
	if reason, ok := hopVerbs[cmd.Verb]; ok {
		return hopHint{Verb: cmd.Verb, Reason: reason}, true
	}
	if cmd.Verb == "sudo" && sudoOpensShell(cmd.Args) {
		return hopHint{Verb: cmd.Verb, Reason: "sudo -i/-s leaves the delegated session's custody for a new user context"}, true
	}
	if containerExecVerbs[cmd.Verb] {
		if subcmd := firstPositional(cmd.Args); subcmd == "exec" {
			return hopHint{
				Verb:   cmd.Verb,
				Reason: fmt.Sprintf("%s exec leaves the delegated session's custody for a new container context", cmd.Verb),
			}, true
		}
	}
	return hopHint{}, false
}

// sudoOpensShell reports whether args requests an interactive shell (-i/-s,
// bare or combined, e.g. "-is") rather than a one-shot command.
func sudoOpensShell(args []out.Arg) bool {
	for _, a := range args {
		if a.HasVar {
			continue
		}
		if sudoInteractiveFlags[a.Value] {
			return true
		}
		if strings.HasPrefix(a.Value, "-") && !strings.HasPrefix(a.Value, "--") &&
			(strings.ContainsRune(a.Value, 'i') || strings.ContainsRune(a.Value, 's')) {
			return true
		}
	}
	return false
}

// firstPositional returns args' first literal positional argument, skipping
// flags and unknown (HasVar) values -- e.g. "docker -H host exec c bash"
// yields "host", not "exec" (a known detection gap for global flags that
// take a value; see hop.go's package doc for the tradeoff).
func firstPositional(args []out.Arg) string {
	for _, a := range args {
		if a.HasVar || strings.HasPrefix(a.Value, "-") {
			continue
		}
		return a.Value
	}
	return ""
}
