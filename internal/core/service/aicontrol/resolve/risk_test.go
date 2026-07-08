package resolve

import (
	"testing"

	"momo-shell/internal/core/port/out"
)

func TestClassify_RiskMatrix(t *testing.T) {
	cases := []struct {
		name     string
		analysis out.CommandAnalysis
		want     RiskLevel
	}{
		{
			name: "rm -rf / -- destructive + critical literal target",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {Value: "/"}}},
			}},
			want: RiskHigh,
		},
		{
			name: "rm -rf ~ -- destructive + critical literal target",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {Value: "~"}}},
			}},
			want: RiskHigh,
		},
		{
			name: "rm -rf /$X -- destructive + variable-dependent target (US-3)",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {HasVar: true}}},
			}},
			want: RiskHigh,
		},
		{
			name: "rm -rf /tmp/x -- destructive but non-critical target",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {Value: "/tmp/x"}}},
			}},
			want: RiskMedium,
		},
		{
			name: "dd of=/tmp/img -- always-destructive, non-critical target",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "dd", Args: []out.Arg{{Value: "of=/tmp/img"}}},
			}},
			want: RiskMedium,
		},
		{
			name: "chmod 755 /tmp/x -- non-recursive, not destructive at all",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "chmod", Args: []out.Arg{{Value: "755"}, {Value: "/tmp/x"}}},
			}},
			want: RiskLow,
		},
		{
			name: "chmod -R 755 /tmp/x -- recursive, non-critical target",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "chmod", Args: []out.Arg{{Value: "-R"}, {Value: "755"}, {Value: "/tmp/x"}}},
			}},
			want: RiskMedium,
		},
		{
			name: "chmod -R 755 / -- recursive + critical target",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "chmod", Args: []out.Arg{{Value: "-R"}, {Value: "755"}, {Value: "/"}}},
			}},
			want: RiskHigh,
		},
		{
			name: "chown -Rf user / -- combined recursive short flag + critical target",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "chown", Args: []out.Arg{{Value: "-Rf"}, {Value: "user"}, {Value: "/"}}},
			}},
			want: RiskHigh,
		},
		{
			name: "echo hi > /dev/sda -- redirect to a raw device is critical regardless of verb",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "echo", Args: []out.Arg{{Value: "hi"}}, Redirects: []out.Arg{{Value: "/dev/sda"}}},
			}},
			want: RiskHigh,
		},
		{
			name: "cmd > /dev/null 2>&1 -- benign pseudo-device, not critical",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "somecmd", Redirects: []out.Arg{{Value: "/dev/null"}}},
			}},
			want: RiskLow,
		},
		{
			name: "ls -la -- ordinary safe command",
			analysis: out.CommandAnalysis{Commands: []out.SimpleCommand{
				{Verb: "ls", Args: []out.Arg{{Value: "-la"}}},
			}},
			want: RiskLow,
		},
		{
			name: "set -o vi -- shell-integrity op forces at least medium",
			analysis: out.CommandAnalysis{IntegrityOps: []string{"set"}},
			want:     RiskMedium,
		},
		{
			name: "rm -rf / AND shell-integrity -- high wins over medium",
			analysis: out.CommandAnalysis{
				Commands:     []out.SimpleCommand{{Verb: "rm", Args: []out.Arg{{Value: "-rf"}, {Value: "/"}}}},
				IntegrityOps: []string{"trap"},
			},
			want: RiskHigh,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := classify(tc.analysis)
			if v.Risk != tc.want {
				t.Errorf("Risk = %v, want %v (reasons: %v)", v.Risk, tc.want, v.Reasons)
			}
			if tc.want != RiskLow && len(v.Reasons) == 0 {
				t.Errorf("expected at least one reason for a non-low verdict")
			}
		})
	}
}

func TestClassify_Uncertain(t *testing.T) {
	v := classify(out.CommandAnalysis{
		Commands:  []out.SimpleCommand{{Verb: "eval", Args: []out.Arg{{HasVar: true}}}},
		Uncertain: true,
	})
	if !v.Uncertain {
		t.Error("expected Uncertain=true to propagate from CommandAnalysis")
	}
	if v.AutoRunnable() {
		t.Error("an uncertain command must never be AutoRunnable, regardless of Risk")
	}
}

func TestClassify_EmptyAnalysisIsLowRisk(t *testing.T) {
	v := classify(out.CommandAnalysis{})
	if v.Risk != RiskLow || v.Uncertain {
		t.Errorf("expected a fully-empty analysis to be low-risk and certain, got %+v", v)
	}
	if len(v.Reasons) != 0 {
		t.Errorf("expected no reasons for an empty analysis, got %v", v.Reasons)
	}
}
