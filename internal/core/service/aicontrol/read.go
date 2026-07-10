package aicontrol

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/service/frame"
)

// ScrollbackReader is the narrow, core-defined interface ReadScrollback needs
// from the scrollback ring buffer -- consumer-side (SessionCreator's
// pattern). scrollback.Service.ReadSince satisfies it structurally.
type ScrollbackReader interface {
	ReadSince(sessionID string, sinceSeq uint64) (data []byte, nextSeq uint64)
}

// OutputMasker is the narrow slice of mask.Masker ReadScrollback consumes --
// a consumer-defined interface so tests can inject a canned redaction
// instead of driving the real 3-layer masker. mask.Service satisfies it
// structurally (this package never imports mask, mirroring custodian's
// import-avoidance for mask.SecretLister).
type OutputMasker interface {
	Apply(sessionID string, chunk []byte) (masked []byte, gated bool, notice string)
}

// ReadScrollback implements in.AIControlUseCase: it projects sessionID's
// scrollback since sinceSeq to an AI client, secret-masked (mask.Apply) and
// trust-framed (frame.Wrap, doc 17 §11 / E4) so the client treats the remote
// bytes as untrusted data, not instructions. Authorization is pairing
// (connection-level token auth, AuthClient) -- read/control are split
// (doc 20 D2): pairing grants read across all sessions, control is the
// separate per-session delegation gate RunCommand enforces. clientID is
// accepted here (matching in.AIControlUseCase) but not consulted for
// authorization; it is reserved for the audit trail (E3).
func (s *Service) ReadScrollback(clientID, sessionID string, sinceSeq uint64) (domain.MaskedChunk, error) {
	data, next := s.scrollback.ReadSince(sessionID, sinceSeq)
	if len(data) == 0 {
		// No new output (or an unknown session -- ReadSince echoes the
		// cursor back for both). An empty <untrusted-remote-output></...>
		// frame would be pure noise on every idle poll, so it's omitted.
		return domain.MaskedChunk{NextSeq: next}, nil
	}

	masked, gated, notice := s.masker.Apply(sessionID, data)
	if gated {
		// Layer-2 withheld secret-dense output. notice is a compile-time
		// system message, not remote bytes -- it is not itself untrusted, so
		// it is not frame-wrapped.
		return domain.MaskedChunk{Data: notice, NextSeq: next}, nil
	}

	// Order matters: mask first (strip secrets), then frame (mark the
	// remaining, already-redacted bytes as untrusted provenance).
	return domain.MaskedChunk{Data: string(frame.Wrap(masked)), NextSeq: next}, nil
}
