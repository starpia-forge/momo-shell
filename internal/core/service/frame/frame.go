// Package frame implements design doc 17 §11's prompt-injection mitigation
// (doc 18 E4): it wraps a session's remote output in an explicit
// trust-boundary marker before it is projected to an AI client, so the
// client/AI treats remote output as untrusted data, not instructions. This
// is a threat *separate* from masking (mask = secrets; frame = provenance),
// hence a separate package. Framing is an always-applied security invariant
// (not an optional, swappable layer), so it is a pure function, not a
// service with a nil-able seam.
package frame

import "bytes"

const (
	openTag  = "<untrusted-remote-output>"
	closeTag = "</untrusted-remote-output>"
)

// Wrap frames remoteOutput as an explicitly-untrusted region. Any literal
// frame delimiter inside remoteOutput is neutralized (angle-bracket escaped)
// so untrusted bytes can neither terminate the frame early nor forge a
// nested boundary -- the result contains exactly one real openTag/closeTag
// pair. Wrap never mutates or aliases its input (bytes.ReplaceAll copies).
func Wrap(remoteOutput []byte) []byte {
	body := defang(remoteOutput)
	var buf bytes.Buffer
	buf.Grow(len(openTag) + len(body) + len(closeTag))
	buf.WriteString(openTag)
	buf.Write(body)
	buf.WriteString(closeTag)
	return buf.Bytes()
}

// defang escapes the angle brackets of any embedded frame delimiter so the
// literal tag can no longer appear verbatim in the body. The two tags are
// not substrings of each other, and the escaped replacement contains no '<',
// so neither replacement can create a new delimiter -- order-independent.
func defang(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte(closeTag), []byte("&lt;/untrusted-remote-output&gt;"))
	b = bytes.ReplaceAll(b, []byte(openTag), []byte("&lt;untrusted-remote-output&gt;"))
	return b
}
