package aicontrol

import (
	"fmt"
	"strings"

	"momo-shell/internal/core/port/out"
)

// interactiveHint is D4a's per-command steering result: a matched
// interactive TUI/REPL/prompt verb and the non-interactive alternative to
// suggest instead of injecting it as-is (design doc 16 §141, doc 17 §357's
// seed ruleset: vim/top/less/py/apt). Static-only, mirroring resolve's L1
// gate -- see resolve.Verdict.Analysis, the parsed a.Commands this operates
// on. Reactive alt-screen handoff (DelegTUIHandoff) and REPL runtime
// detection are out of scope for this slice (doc 16 §167 still lists the
// REPL detection primitive as undesigned) -- see doc 23.
type interactiveHint struct {
	Verb       string
	Suggestion string
}

// editorVerbs launch a full-screen modal editor -- always interactive,
// regardless of arguments.
var editorVerbs = map[string]string{
	"vim":   "write the file non-interactively instead (here-doc/redirection, or sed/tee for edits)",
	"vi":    "write the file non-interactively instead (here-doc/redirection, or sed/tee for edits)",
	"nvim":  "write the file non-interactively instead (here-doc/redirection, or sed/tee for edits)",
	"nano":  "write the file non-interactively instead (here-doc/redirection, or sed/tee for edits)",
	"emacs": "write the file non-interactively instead (here-doc/redirection, or sed/tee for edits)",
}

// pagerVerbs page output a screen at a time -- always interactive.
var pagerVerbs = map[string]string{
	"less": "read it non-interactively instead (cat, or head/tail/sed -n for a slice)",
	"more": "read it non-interactively instead (cat, or head/tail/sed -n for a slice)",
}

// monitorVerbs refresh a full-screen live view -- always interactive.
var monitorVerbs = map[string]string{
	"top":  "take a one-shot snapshot instead (ps aux, or `top -b -n1`)",
	"htop": "take a one-shot snapshot instead (ps aux, or `htop -b -n1` if supported)",
}

// replVerbs are interactive only when launched with no script/expression
// argument -- isREPLLaunch decides.
var replVerbs = map[string]string{
	"python":  "run it non-interactively instead (`python -c '...'` or a script file)",
	"python3": "run it non-interactively instead (`python3 -c '...'` or a script file)",
}

// pkgMgrVerbs prompt for confirmation on mutating subcommands unless an
// auto-yes flag is given -- pkgMgrPrompts decides.
var pkgMgrVerbs = map[string]bool{"apt": true, "apt-get": true}

var pkgMgrMutatingSubcommands = map[string]bool{
	"install": true, "remove": true, "purge": true, "upgrade": true,
	"dist-upgrade": true, "full-upgrade": true, "autoremove": true,
}

var pkgMgrYesFlags = map[string]bool{"-y": true, "--yes": true, "--assume-yes": true}

// replFlags take a script/expression as their own argument rather than
// dropping into a REPL.
var replFlags = map[string]bool{"-c": true, "-m": true}

// detectInteractive scans a parsed command's pipeline stages, in order,
// for the first verb that would drop the AI into an interactive TUI/REPL/
// prompt instead of running to completion (design doc 16 §141, FR-9 ③④).
// a.Commands is empty when static analysis couldn't parse the command
// (unsupported dialect, parse failure) -- that already degrades to the
// existing approval gate via Verdict.Uncertain, so this reports no match
// rather than guessing.
func detectInteractive(a out.CommandAnalysis) (interactiveHint, bool) {
	for _, cmd := range a.Commands {
		if hint, ok := classifyInteractive(cmd); ok {
			return hint, true
		}
	}
	return interactiveHint{}, false
}

func classifyInteractive(cmd out.SimpleCommand) (interactiveHint, bool) {
	if suggestion, ok := editorVerbs[cmd.Verb]; ok {
		return interactiveHint{Verb: cmd.Verb, Suggestion: suggestion}, true
	}
	if suggestion, ok := pagerVerbs[cmd.Verb]; ok {
		return interactiveHint{Verb: cmd.Verb, Suggestion: suggestion}, true
	}
	if suggestion, ok := monitorVerbs[cmd.Verb]; ok {
		return interactiveHint{Verb: cmd.Verb, Suggestion: suggestion}, true
	}
	if suggestion, ok := replVerbs[cmd.Verb]; ok && isREPLLaunch(cmd.Args) {
		return interactiveHint{Verb: cmd.Verb, Suggestion: suggestion}, true
	}
	if pkgMgrVerbs[cmd.Verb] {
		if subcmd, prompts := pkgMgrPrompts(cmd.Args); prompts {
			return interactiveHint{
				Verb:       cmd.Verb,
				Suggestion: fmt.Sprintf("add -y/--yes to auto-confirm the %q prompt", subcmd),
			}, true
		}
	}
	return interactiveHint{}, false
}

// isREPLLaunch reports whether args launches an interactive REPL rather
// than running a script/expression non-interactively: true only when no
// -c/-m flag (bare or combined, e.g. "-uc") and no literal positional
// argument is present. A HasVar argument is skipped rather than treated as
// a positional -- an unknown value shouldn't be read as proof either way
// here, mirroring resolve/risk.go's conservative-elsewhere handling.
func isREPLLaunch(args []out.Arg) bool {
	for _, a := range args {
		if a.HasVar {
			continue
		}
		if strings.HasPrefix(a.Value, "-") {
			if replFlagSet(a.Value) {
				return false
			}
			continue
		}
		return false // a literal positional -- a script/expression argument
	}
	return true
}

func replFlagSet(flag string) bool {
	if replFlags[flag] {
		return true
	}
	// Combined short flags, e.g. "-uc" (unbuffered + command).
	return strings.HasPrefix(flag, "-") && !strings.HasPrefix(flag, "--") &&
		(strings.ContainsRune(flag, 'c') || strings.ContainsRune(flag, 'm'))
}

// pkgMgrPrompts reports whether args' first literal positional subcommand
// mutates the system and no auto-yes flag is present -- e.g.
// "apt install foo" prompts, "apt install -y foo" and "apt list" don't.
func pkgMgrPrompts(args []out.Arg) (subcommand string, prompts bool) {
	yes := false
	for _, a := range args {
		if a.HasVar || strings.HasPrefix(a.Value, "-") {
			if pkgMgrYesFlags[a.Value] {
				yes = true
			}
			continue
		}
		if subcommand == "" {
			subcommand = a.Value
		}
	}
	return subcommand, pkgMgrMutatingSubcommands[subcommand] && !yes
}

// InteractiveCommandError is RunCommand's rejection when detectInteractive
// matches -- distinct from the sentinel Err* vars (ErrCommandDenied etc.)
// because it carries a per-command Suggestion the AI can act on, not just
// a fixed message. Once run_command becomes an MCP tool (A-track), its
// closure maps this to a CallToolResult{IsError:true} with Suggestion as
// the advisory text (mirroring dispatch.go's textResult shape) rather than
// a bare error string.
type InteractiveCommandError struct {
	Verb       string
	Suggestion string
}

func (e *InteractiveCommandError) Error() string {
	return fmt.Sprintf("aicontrol: %q is interactive (TUI/REPL/prompt) -- %s", e.Verb, e.Suggestion)
}
