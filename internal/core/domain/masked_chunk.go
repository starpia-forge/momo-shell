package domain

// MaskedChunk is a masked slice of a session's scrollback returned to an AI
// client via ReadScrollback. Masking itself is core/service/mask's
// responsibility (design doc 17 §7, FR-6); this type only fixes the shape
// callers exchange.
type MaskedChunk struct {
	Data    string `json:"data"`
	NextSeq uint64 `json:"nextSeq"`
}
