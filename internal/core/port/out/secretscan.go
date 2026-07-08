package out

// SecretScanner is the driven port for finding likely-secret substrings in
// arbitrary text (design doc 17 §7.2, doc 18 E1). Pure -- no I/O, no session
// access; internal/core/service/mask (E2) applies the actual redaction on
// top of what this reports.
type SecretScanner interface {
	Scan(text []byte) []SecretHit
}

// SecretHit is one match: what kind, where, and the exact substring (E2
// needs Value for allowlist comparison / custodian cross-check, not just
// the span).
type SecretHit struct {
	Type  string // rule identifier, e.g. "aws-access-key-id", "private-key-block", "generic-high-entropy"
	Start int    // byte offset into the scanned text where the match begins
	End   int    // byte offset where the match ends (exclusive)
	Value string // the matched substring
}
