package resolve

import (
	"strings"

	"momo-shell/internal/core/port/out"
)

// alwaysDestructive verbs are destructive regardless of arguments (design
// doc 17 §6.1 step 3's risk matrix).
var alwaysDestructive = map[string]bool{"rm": true, "dd": true, "mkfs": true}

// recursiveOnlyDestructive verbs are destructive only when given a
// recursive flag (doc 17 §6.1: "chmod -R/chown -R" specifically, not bare
// chmod/chown).
var recursiveOnlyDestructive = map[string]bool{"chmod": true, "chown": true}

var recursiveFlags = map[string]bool{"-R": true, "-r": true, "--recursive": true}

// benignDevicePaths are well-known pseudo-devices used constantly for
// harmless stream redirection (e.g. "cmd > /dev/null 2>&1" to silence
// output) -- treating all of "/dev/*" as critical (as doc 17's matrix
// literally lists it) would flag one of the most common, harmless shell
// idioms. The matrix's intent is raw block/char devices (/dev/sda,
// /dev/nvme0n1, ...), not these.
var benignDevicePaths = map[string]bool{
	"/dev/null": true, "/dev/zero": true, "/dev/tty": true,
	"/dev/stdin": true, "/dev/stdout": true, "/dev/stderr": true,
}

// classify applies the risk matrix (doc 17 §6.1 step 3), uncertainty (step
// 4), and shell-integrity (step 5) rules to a parsed command.
func classify(a out.CommandAnalysis) Verdict {
	v := Verdict{Risk: RiskLow}

	if len(a.IntegrityOps) > 0 {
		v.Risk = maxRisk(v.Risk, RiskMedium)
		v.Reasons = append(v.Reasons, "shell-integrity operation: "+strings.Join(a.IntegrityOps, ", "))
	}

	for _, cmd := range a.Commands {
		if reason, ok := criticalRedirectTarget(cmd); ok {
			v.Risk = RiskHigh
			v.Reasons = append(v.Reasons, cmd.Verb+": redirects output to "+reason)
		}
		if isDestructiveVerb(cmd) {
			if reason, ok := criticalArgTarget(cmd); ok {
				v.Risk = RiskHigh
				v.Reasons = append(v.Reasons, cmd.Verb+": destructive command targeting "+reason)
			} else {
				v.Risk = maxRisk(v.Risk, RiskMedium)
				v.Reasons = append(v.Reasons, cmd.Verb+": destructive command")
			}
		}
	}

	if a.Uncertain {
		v.Uncertain = true
		v.Reasons = append(v.Reasons, "command contains a dynamically-determined element ($(...), eval, or a dynamic verb)")
	}

	return v
}

func isDestructiveVerb(cmd out.SimpleCommand) bool {
	if alwaysDestructive[cmd.Verb] {
		return true
	}
	if recursiveOnlyDestructive[cmd.Verb] {
		return hasRecursiveFlag(cmd.Args)
	}
	return false
}

func hasRecursiveFlag(args []out.Arg) bool {
	for _, a := range args {
		if recursiveFlags[a.Value] {
			return true
		}
		// Combined short flags, e.g. "-Rf"/"-fR".
		if strings.HasPrefix(a.Value, "-") && !strings.HasPrefix(a.Value, "--") && strings.ContainsRune(a.Value, 'R') {
			return true
		}
	}
	return false
}

func criticalArgTarget(cmd out.SimpleCommand) (string, bool) {
	for _, a := range cmd.Args {
		if reason, ok := criticalTarget(a); ok {
			return reason, true
		}
	}
	return "", false
}

func criticalRedirectTarget(cmd out.SimpleCommand) (string, bool) {
	for _, r := range cmd.Redirects {
		if reason, ok := criticalTarget(r); ok {
			return reason, true
		}
	}
	return "", false
}

// criticalTarget implements the risk matrix's "critical target" set: /, ~,
// a variable-dependent value (could resolve empty/unset -- US-3's core
// case), or a raw device path.
func criticalTarget(a out.Arg) (string, bool) {
	switch {
	case a.HasVar:
		return "a variable-dependent path (value unknown -- could be empty or unset)", true
	case a.Value == "/" || a.Value == "~":
		return "\"" + a.Value + "\"", true
	case strings.HasPrefix(a.Value, "/dev/") && !benignDevicePaths[a.Value]:
		return "a device path (" + a.Value + ")", true
	default:
		return "", false
	}
}

var riskRank = map[RiskLevel]int{RiskLow: 0, RiskMedium: 1, RiskHigh: 2}

func maxRisk(a, b RiskLevel) RiskLevel {
	if riskRank[b] > riskRank[a] {
		return b
	}
	return a
}
