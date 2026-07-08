// Package secretscan implements out.SecretScanner: a small curated set of
// high-confidence regex rules for well-known secret formats, plus a
// Shannon-entropy heuristic for generic high-entropy strings the fixed
// rules miss (design doc 17 §7.2, doc 18 E1, DR-5). Pure function over
// text -- no session/IO dependency, mirrors shparse's adapter shape: no
// third-party ruleset crosses this boundary, only the plain-data
// out.SecretHit.
package secretscan

import (
	"math"
	"regexp"

	"momo-shell/internal/core/port/out"
)

// Scanner implements out.SecretScanner.
type Scanner struct {
	entropyThreshold float64
}

// defaultEntropyThreshold is a starter estimate (bits/byte) for flagging a
// candidate token as likely-random. Tuning against real session output is
// an open task (DR-5) -- see the E1 plan's gap notes.
const defaultEntropyThreshold = 4.5

func New() *Scanner {
	return &Scanner{entropyThreshold: defaultEntropyThreshold}
}

// NewWithEntropyThreshold lets a caller override the default for DR-5's
// false-positive/false-negative tuning.
func NewWithEntropyThreshold(threshold float64) *Scanner {
	return &Scanner{entropyThreshold: threshold}
}

var _ out.SecretScanner = (*Scanner)(nil)

// rule is one curated regex pattern paired with the SecretHit.Type it
// reports.
type rule struct {
	typ     string
	pattern *regexp.Regexp
}

// rules is the curated starter set (DR-5: hand-picked high-confidence
// fixed-format secrets, not an embedded gitleaks ruleset -- see the E1
// plan's rationale for choosing curation over embedding).
var rules = []rule{
	{"aws-access-key-id", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"private-key-block", regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`)},
	{"github-token", regexp.MustCompile(`gh[pousr]_[0-9A-Za-z]{36,255}`)},
	{"slack-token", regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{10,72}`)},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)},
}

// entropyCandidate matches contiguous runs of base64/hex-ish characters
// long enough to be worth an entropy check.
var entropyCandidate = regexp.MustCompile(`[A-Za-z0-9+/=_.-]{20,}`)

func (s *Scanner) Scan(text []byte) []out.SecretHit {
	hits := s.scanRules(text)
	hits = append(hits, s.scanEntropy(text, hits)...)
	return hits
}

func (s *Scanner) scanRules(text []byte) []out.SecretHit {
	var hits []out.SecretHit
	for _, r := range rules {
		for _, loc := range r.pattern.FindAllIndex(text, -1) {
			hits = append(hits, out.SecretHit{
				Type:  r.typ,
				Start: loc[0],
				End:   loc[1],
				Value: string(text[loc[0]:loc[1]]),
			})
		}
	}
	return hits
}

// scanEntropy flags high-entropy candidate tokens not already covered by a
// curated rule hit -- ruleHits is checked so the same underlying secret
// isn't double-reported (e.g. a token a rule already matched).
func (s *Scanner) scanEntropy(text []byte, ruleHits []out.SecretHit) []out.SecretHit {
	var hits []out.SecretHit
	for _, loc := range entropyCandidate.FindAllIndex(text, -1) {
		start, end := loc[0], loc[1]
		if overlapsAny(start, end, ruleHits) {
			continue
		}
		candidate := text[start:end]
		if shannonEntropy(candidate) >= s.entropyThreshold {
			hits = append(hits, out.SecretHit{
				Type:  "generic-high-entropy",
				Start: start,
				End:   end,
				Value: string(candidate),
			})
		}
	}
	return hits
}

func overlapsAny(start, end int, hits []out.SecretHit) bool {
	for _, h := range hits {
		if start < h.End && end > h.Start {
			return true
		}
	}
	return false
}

// shannonEntropy computes the Shannon entropy (bits/byte) of data's byte
// distribution -- a pure statistical measure, no external dependency.
func shannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	entropy := 0.0
	n := float64(len(data))
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		entropy -= p * math.Log2(p)
	}
	return entropy
}
