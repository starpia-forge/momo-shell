// Package mask implements design doc 17 §7.2's 3-layer AI-facing masking
// (doc 18 E2). Egress projection (ReadScrollback reading from a live
// per-session ring buffer, and real SecretLister/CommandContextProvider
// implementations) is E2 2부 -- deferred because the ring buffer itself
// doesn't exist yet (a cycle-size split, not a data-source block, unlike
// B4/D3). This package is layer 3 fully live (consumes E1's
// out.SecretScanner) plus layer 1/2 logic against consumer-defined
// interfaces that nothing implements in production yet.
package mask

import (
	"bytes"
	"sort"
	"strings"

	"momo-shell/internal/core/port/out"
)

// Masker is the AI-facing output-masking contract (design doc 17 §7.2).
type Masker interface {
	Apply(sessionID string, chunk []byte) (masked []byte, gated bool, notice string)
}

// SecretLister supplies the custodian's known-secret values for a session
// (layer 1, doc 17 §6.3 -- e.g. the host's stored SSH password). A
// consumer-defined interface (aicontrol.SessionCreator's pattern) --
// production wiring (aicontrol -> SecretStore, sessionID -> HostID) is E2
// 2부's job.
type SecretLister interface {
	SecretsForSession(sessionID string) []string
}

// CommandContextProvider supplies the session's current in-flight
// command's verb/args (layer 2). aicontrol already tracks the command
// string via D1/C4's commands map, so wiring this is comparatively small,
// but is still E2 2부's job.
type CommandContextProvider interface {
	CurrentCommand(sessionID string) (verb string, args []string, ok bool)
}

// Deps are the collaborators Service needs. Secrets/Commands are optional
// (nil disables that layer rather than panicking) since nothing implements
// them in production yet.
type Deps struct {
	Scanner  out.SecretScanner      // required -- layer 3
	Secrets  SecretLister           // optional; nil => layer 1 no-op
	Commands CommandContextProvider // optional; nil => layer 2 no-op
}

type Service struct {
	scanner  out.SecretScanner
	secrets  SecretLister
	commands CommandContextProvider
}

func New(deps Deps) *Service {
	return &Service{scanner: deps.Scanner, secrets: deps.Secrets, commands: deps.Commands}
}

var _ Masker = (*Service)(nil)

// Apply runs chunk through all three layers in order: layer 2 (command-aware
// gating) can withhold the entire chunk outright; otherwise layer 1
// (custodian exact-match) strips known secret values, then layer 3
// (curated ruleset + entropy, via out.SecretScanner) redacts what's left.
func (s *Service) Apply(sessionID string, chunk []byte) (masked []byte, gated bool, notice string) {
	if s.commands != nil {
		if verb, args, ok := s.commands.CurrentCommand(sessionID); ok && isSecretDenseCommand(verb, args) {
			return nil, true, "secret-dense command output withheld"
		}
	}

	result := append([]byte(nil), chunk...)

	if s.secrets != nil {
		for _, secret := range s.secrets.SecretsForSession(sessionID) {
			if secret == "" {
				continue
			}
			result = bytes.ReplaceAll(result, []byte(secret), []byte("[REDACTED:custodian]"))
		}
	}

	hits := s.scanner.Scan(result)
	return redact(result, hits), false, ""
}

// isSecretDenseCommand implements doc 17 §7.2 layer 2's identification
// list: env/set (dump all vars), cat targeting dotfiles/credentials, gpg,
// kubectl get secret.
func isSecretDenseCommand(verb string, args []string) bool {
	switch verb {
	case "env", "set", "gpg":
		return true
	case "cat":
		for _, a := range args {
			if matchesSecretPath(a) {
				return true
			}
		}
		return false
	case "kubectl":
		return len(args) >= 2 && args[0] == "get" && args[1] == "secret"
	}
	return false
}

func matchesSecretPath(arg string) bool {
	lower := strings.ToLower(arg)
	return strings.Contains(lower, ".ssh/") || strings.HasSuffix(lower, ".env") || strings.Contains(lower, "credentials")
}

// redact renders each hit as a visible [REDACTED:type] marker (doc 17 §7.2
// usability item). E1's SecretHits never overlap, so a start-offset sort is
// enough to replace them in order.
func redact(text []byte, hits []out.SecretHit) []byte {
	if len(hits) == 0 {
		return text
	}
	sorted := append([]out.SecretHit(nil), hits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })

	var buf bytes.Buffer
	last := 0
	for _, h := range sorted {
		if h.Start < last {
			continue // defensive -- E1 shouldn't produce overlaps, but never write out-of-order
		}
		buf.Write(text[last:h.Start])
		buf.WriteString("[REDACTED:" + h.Type + "]")
		last = h.End
	}
	buf.Write(text[last:])
	return buf.Bytes()
}
