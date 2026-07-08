package resolve

import (
	"strings"
	"testing"

	"momo-shell/internal/core/port/out"
)

func TestBuildGuardedCommand(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		analysis out.CommandAnalysis
		want     string
	}{
		{
			name:    "rm -rf /$X -- destructive verb with variable-dependent target (US-3)",
			command: "rm -rf /$X",
			analysis: out.CommandAnalysis{
				Commands: []out.SimpleCommand{
					{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {HasVar: true}}},
				},
				ReferencedVars: []string{"X"},
			},
			want: `if [ -n "${X:-}" ]; then rm -rf /$X; else echo momo:abort; fi`,
		},
		{
			name:    "rm -rf /$A/$B -- multiple referenced vars, all guarded in first-occurrence order",
			command: "rm -rf /$A/$B",
			analysis: out.CommandAnalysis{
				Commands: []out.SimpleCommand{
					{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {HasVar: true}}},
				},
				ReferencedVars: []string{"A", "B"},
			},
			want: `if [ -n "${A:-}" ] && [ -n "${B:-}" ]; then rm -rf /$A/$B; else echo momo:abort; fi`,
		},
		{
			name:    "rm -rf / -- literal-only destructive target, nothing to re-check",
			command: "rm -rf /",
			analysis: out.CommandAnalysis{
				Commands: []out.SimpleCommand{
					{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {Value: "/"}}},
				},
			},
			want: "",
		},
		{
			name:    "ls -la -- safe command, no guard",
			command: "ls -la",
			analysis: out.CommandAnalysis{
				Commands: []out.SimpleCommand{{Verb: "ls", Args: []out.Arg{{Value: "-la"}}}},
			},
			want: "",
		},
		{
			name:    "chmod -R 755 /$DIR -- recursive-destructive with variable target",
			command: "chmod -R 755 /$DIR",
			analysis: out.CommandAnalysis{
				Commands: []out.SimpleCommand{
					{Verb: "chmod", Args: []out.Arg{{Value: "-R"}, {Value: "755"}, {HasVar: true}}},
				},
				ReferencedVars: []string{"DIR"},
			},
			want: `if [ -n "${DIR:-}" ]; then chmod -R 755 /$DIR; else echo momo:abort; fi`,
		},
		{
			name:    "chmod 755 /$DIR -- non-recursive chmod is not destructive, no guard",
			command: "chmod 755 /$DIR",
			analysis: out.CommandAnalysis{
				Commands: []out.SimpleCommand{
					{Verb: "chmod", Args: []out.Arg{{Value: "755"}, {HasVar: true}}},
				},
				ReferencedVars: []string{"DIR"},
			},
			want: "",
		},
		{
			name:    "dd of=$OUT -- always-destructive with variable target",
			command: "dd of=$OUT",
			analysis: out.CommandAnalysis{
				Commands: []out.SimpleCommand{
					{Verb: "dd", Args: []out.Arg{{Value: "of=", HasVar: true}}},
				},
				ReferencedVars: []string{"OUT"},
			},
			want: `if [ -n "${OUT:-}" ]; then dd of=$OUT; else echo momo:abort; fi`,
		},
		{
			name:    "eval $CMD -- uncertain but not a destructive verb, no guard",
			command: "eval $CMD",
			analysis: out.CommandAnalysis{
				Commands:       []out.SimpleCommand{{Verb: "eval", Args: []out.Arg{{HasVar: true}}}},
				ReferencedVars: []string{"CMD"},
				Uncertain:      true,
			},
			want: "",
		},
		{
			name:     "empty analysis -- no guard",
			command:  "",
			analysis: out.CommandAnalysis{},
			want:     "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildGuardedCommand(tc.command, tc.analysis)
			if got != tc.want {
				t.Errorf("buildGuardedCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildGuardedCommand_Invariants pins the safety properties the guard's
// chosen syntax must uphold -- see the doc comment on buildGuardedCommand
// for why the design doc's literal `exit`-based example was rejected.
func TestBuildGuardedCommand_Invariants(t *testing.T) {
	analysis := out.CommandAnalysis{
		Commands: []out.SimpleCommand{
			{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {HasVar: true}}},
		},
		ReferencedVars: []string{"EMPTY"},
	}
	got := buildGuardedCommand("rm -rf /$EMPTY", analysis)

	if strings.Contains(got, "exit") {
		t.Errorf("guarded command must never contain \"exit\" -- it would terminate the shared shell session: %q", got)
	}
	if !strings.HasPrefix(got, "if ") {
		t.Errorf("guarded command must be an if/then/else/fi compound (no subshell), got %q", got)
	}
	if !strings.HasSuffix(got, "; else echo "+abortMarker+"; fi") {
		t.Errorf("guarded command must end with the abort branch, got %q", got)
	}
	if !strings.Contains(got, "rm -rf /$EMPTY") {
		t.Errorf("guarded command must embed the original command verbatim, got %q", got)
	}
}
