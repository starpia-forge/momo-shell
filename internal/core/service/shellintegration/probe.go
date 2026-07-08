package shellintegration

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ErrNotAtPrompt is returned by Query when the session's shell isn't
// currently sitting at a fresh prompt with confirmed hooks (see AtPrompt) --
// injecting a probe outside that window risks landing mid-command or
// mid-TUI (doc 17 §6.2: "프롬프트 상태에서만 주입").
var ErrNotAtPrompt = errors.New("shellintegration: session not at a prompt")

// VarValue is one shell variable's live state, as reported by a VarQuery
// probe. Set distinguishes "never assigned" from "assigned" -- Value is
// only meaningful when Set is true, and is the exact byte-for-byte
// original value (base64 round-tripped, so empty and whitespace-only
// values are preserved and distinguishable from unset).
type VarValue struct {
	Set   bool
	Value string
}

// Query runs a VarQuery probe (doc 17 §6.2, verified payloads in doc 19
// §3): it injects a script into the session's shell that reports, for each
// name, whether it is unset or set (and if set, its exact value), then
// waits for the probe's own OSC1337 reply to be parsed out of the
// session's output stream. The probe never touches shell history or
// disturbs $?/$LASTEXITCODE (see the per-dialect ProbeScript templates).
//
// Returns ErrSessionNotFound if this service was never Attach-ed to
// sessionID, ErrNotAtPrompt if the shell isn't currently idle at a
// hooks-confirmed prompt, or ctx's error if the reply doesn't arrive
// before ctx is done -- callers should treat a timeout as "value
// unknowable" (doc 19 §4.1: this is also the signal for values too large
// for the hosting pipeline's relay buffer) and degrade to a static,
// approval-required verdict rather than retrying indefinitely.
func (s *Service) Query(ctx context.Context, sessionID string, names []string) (map[string]VarValue, error) {
	for _, name := range names {
		if !validVarName(name) {
			return nil, fmt.Errorf("shellintegration: invalid variable name %q", name)
		}
	}

	st := s.get(sessionID)
	if st == nil {
		return nil, ErrSessionNotFound
	}

	st.mu.Lock()
	if st.phase != phaseActive || !st.atPrompt {
		st.mu.Unlock()
		return nil, ErrNotAtPrompt
	}
	dialect := st.dialect
	st.mu.Unlock()

	nonce := newNonce()
	script, err := s.builder.ProbeScript(dialect, nonce, names)
	if err != nil {
		return nil, err
	}

	ch := make(chan string, 1)
	s.probeMu.Lock()
	s.probes[nonce] = ch
	s.probeMu.Unlock()
	defer func() {
		s.probeMu.Lock()
		delete(s.probes, nonce)
		s.probeMu.Unlock()
	}()

	if err := s.shell.WriteRaw(sessionID, script); err != nil {
		return nil, err
	}

	select {
	case payload := <-ch:
		return decodeProbePayload(payload), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// routeProbe delivers a parsed probe reply to the Query call waiting on its
// nonce, if any (a reply for a nonce nobody is waiting on -- e.g. Query
// already timed out -- is silently dropped). Called from dispatch, which
// (per OnOutput's contract) never holds sessionState.mu while dispatching,
// so this only ever needs probeMu.
func (s *Service) routeProbe(nonce, payload string) {
	s.probeMu.Lock()
	ch, ok := s.probes[nonce]
	if ok {
		delete(s.probes, nonce)
	}
	s.probeMu.Unlock()
	if ok {
		ch <- payload
	}
}

// decodeProbePayload parses a probe reply body of the form
// "NAME=flag:b64value;NAME2=flag:b64value;" (doc 19 §3: flag is "0" for
// unset, "1" for set with the value base64-encoded). Entries that don't
// parse cleanly are dropped rather than fabricated -- a name absent from
// the returned map means "undeterminable", which callers should treat with
// the same caution as an unset variable.
func decodeProbePayload(payload string) map[string]VarValue {
	result := make(map[string]VarValue)
	for _, entry := range strings.Split(payload, ";") {
		if entry == "" {
			continue
		}
		name, rest, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		flag, b64Value, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		switch flag {
		case "0":
			result[name] = VarValue{Set: false}
		case "1":
			raw, err := base64.StdEncoding.DecodeString(b64Value)
			if err != nil {
				continue
			}
			result[name] = VarValue{Set: true, Value: string(raw)}
		}
	}
	return result
}

// validVarName reports whether name is safe to interpolate into a probe
// script as a bare shell identifier -- the trust boundary for Query's
// input (doc plan D4). Restricting to the POSIX shell-variable-name
// grammar (letter/underscore, then letters/digits/underscores) rules out
// any injection via the variable-name channel itself; the queried values
// never need this treatment (always base64-encoded, never interpolated).
func validVarName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			continue
		case i > 0 && r >= '0' && r <= '9':
			continue
		default:
			return false
		}
	}
	return true
}
