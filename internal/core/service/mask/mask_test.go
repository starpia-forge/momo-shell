package mask

import (
	"strings"
	"testing"

	"momo-shell/internal/adapter/out/secretscan"
)

// fakeSecretLister is a hand-rolled SecretLister (house style: no mock
// framework -- cf. aicontrol/mocks_test.go's fakeResolver).
type fakeSecretLister struct {
	secrets []string
}

func (f *fakeSecretLister) SecretsForSession(sessionID string) []string {
	return f.secrets
}

// fakeCommandContext is a hand-rolled CommandContextProvider.
type fakeCommandContext struct {
	verb string
	args []string
	ok   bool
}

func (f *fakeCommandContext) CurrentCommand(sessionID string) (verb string, args []string, ok bool) {
	return f.verb, f.args, f.ok
}

func TestApply_Layer3Only_SafeTextUnchanged(t *testing.T) {
	s := New(Deps{Scanner: secretscan.New()})

	masked, gated, notice := s.Apply("sess-1", []byte("ls -la /home/user && echo done"))

	if gated || notice != "" {
		t.Fatalf("Apply() = gated=%v notice=%q, want ungated with no notice", gated, notice)
	}
	if string(masked) != "ls -la /home/user && echo done" {
		t.Errorf("masked = %q, want unchanged safe text", masked)
	}
}

func TestApply_Layer3Only_RedactsCuratedSecret(t *testing.T) {
	s := New(Deps{Scanner: secretscan.New()})

	masked, gated, _ := s.Apply("sess-1", []byte("export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"))

	if gated {
		t.Fatal("Apply() gated a plain export command, want ungated")
	}
	want := "export AWS_ACCESS_KEY_ID=[REDACTED:aws-access-key-id]"
	if string(masked) != want {
		t.Errorf("masked = %q, want %q", masked, want)
	}
}

func TestApply_Layer1_CustodianExactMatchRedacted(t *testing.T) {
	s := New(Deps{
		Scanner: secretscan.New(),
		Secrets: &fakeSecretLister{secrets: []string{"hunter2"}},
	})

	masked, gated, _ := s.Apply("sess-1", []byte("password is hunter2 here"))

	if gated {
		t.Fatal("Apply() gated ordinary output, want ungated")
	}
	want := "password is [REDACTED:custodian] here"
	if string(masked) != want {
		t.Errorf("masked = %q, want %q", masked, want)
	}
}

func TestApply_Layer1AndLayer3Combined(t *testing.T) {
	s := New(Deps{
		Scanner: secretscan.New(),
		Secrets: &fakeSecretLister{secrets: []string{"hunter2"}},
	})

	masked, _, _ := s.Apply("sess-1", []byte("pwd=hunter2 key=AKIAIOSFODNN7EXAMPLE"))

	got := string(masked)
	if !strings.Contains(got, "[REDACTED:custodian]") {
		t.Errorf("masked = %q, want a custodian redaction", got)
	}
	if !strings.Contains(got, "[REDACTED:aws-access-key-id]") {
		t.Errorf("masked = %q, want an aws-access-key-id redaction", got)
	}
	if strings.Contains(got, "hunter2") || strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("masked = %q, want neither raw secret to survive", got)
	}
}

func TestApply_Layer2Gating_UnconditionalVerbsAlwaysGate(t *testing.T) {
	for _, verb := range []string{"env", "set", "gpg"} {
		t.Run(verb, func(t *testing.T) {
			s := New(Deps{
				Scanner:  secretscan.New(),
				Commands: &fakeCommandContext{verb: verb, args: []string{"whatever"}, ok: true},
			})

			masked, gated, notice := s.Apply("sess-1", []byte("some output"))

			if !gated || masked != nil || notice == "" {
				t.Errorf("Apply() = masked=%q gated=%v notice=%q, want gated=true masked=nil notice!=\"\"", masked, gated, notice)
			}
		})
	}
}

func TestApply_Layer2Gating_CatSecretPath(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantGate bool
	}{
		{"targets .ssh directory", []string{"/home/user/.ssh/id_rsa"}, true},
		{"targets .env file", []string{"/app/.env"}, true},
		{"targets credentials file", []string{"~/aws/credentials"}, true},
		{"targets an ordinary file", []string{"/etc/hosts"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := New(Deps{
				Scanner:  secretscan.New(),
				Commands: &fakeCommandContext{verb: "cat", args: c.args, ok: true},
			})

			_, gated, _ := s.Apply("sess-1", []byte("file contents"))

			if gated != c.wantGate {
				t.Errorf("Apply() gated = %v, want %v", gated, c.wantGate)
			}
		})
	}
}

func TestApply_Layer2Gating_KubectlGetSecretOnly(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantGate bool
	}{
		{"get secret", []string{"get", "secret", "db-creds"}, true},
		{"get pods", []string{"get", "pods"}, false},
		{"describe secret", []string{"describe", "secret", "db-creds"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := New(Deps{
				Scanner:  secretscan.New(),
				Commands: &fakeCommandContext{verb: "kubectl", args: c.args, ok: true},
			})

			_, gated, _ := s.Apply("sess-1", []byte("kubectl output"))

			if gated != c.wantGate {
				t.Errorf("Apply() gated = %v, want %v", gated, c.wantGate)
			}
		})
	}
}

func TestApply_Layer2Gating_NoCurrentCommandNeverGates(t *testing.T) {
	s := New(Deps{
		Scanner:  secretscan.New(),
		Commands: &fakeCommandContext{ok: false}, // no in-flight command
	})

	masked, gated, _ := s.Apply("sess-1", []byte("ordinary output"))

	if gated || string(masked) != "ordinary output" {
		t.Errorf("Apply() = masked=%q gated=%v, want ungated passthrough when there's no current command", masked, gated)
	}
}

func TestApply_NilSecretsAndCommands_Layer3OnlyStillWorks(t *testing.T) {
	// Deps{Scanner: ...} alone -- Secrets and Commands both nil, mirroring
	// production before E2 2부 wires real implementations.
	s := New(Deps{Scanner: secretscan.New()})

	masked, gated, notice := s.Apply("sess-1", []byte("export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"))

	if gated || notice != "" {
		t.Fatalf("Apply() = gated=%v notice=%q, want ungated with nil Secrets/Commands", gated, notice)
	}
	want := "export AWS_ACCESS_KEY_ID=[REDACTED:aws-access-key-id]"
	if string(masked) != want {
		t.Errorf("masked = %q, want %q", masked, want)
	}
}

func TestApply_MultipleHitsRenderedInOrder(t *testing.T) {
	s := New(Deps{Scanner: secretscan.New()})
	text := "AKIAIOSFODNN7EXAMPLE and also ghp_abcdefghijklmnopqrstuvwxyz1234567890 end"

	masked, _, _ := s.Apply("sess-1", []byte(text))

	want := "[REDACTED:aws-access-key-id] and also [REDACTED:github-token] end"
	if string(masked) != want {
		t.Errorf("masked = %q, want %q", masked, want)
	}
}
