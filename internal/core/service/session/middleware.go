package session

import "momo-shell/internal/core/domain"

// OutputMiddleware may rewrite or suppress each raw output chunk before it
// is recorded by CommandTap, coalesced, and published to the frontend --
// used by the transfer service to detect and intercept ZMODEM (rz/sz)
// traffic so protocol bytes never reach the terminal or command history. A
// nil middleware (the default) leaves output untouched.
type OutputMiddleware interface {
	// Attach registers a newly-running session. Called for every session
	// (local and SSH) -- implementations that only care about SSH should
	// check kind themselves.
	Attach(sessionID string, kind domain.SessionKind)
	// OnOutput returns the bytes that should continue on to CommandTap and
	// the frontend. Returning nil/empty suppresses the chunk entirely.
	// Called from the pump goroutine only, so implementations must not block
	// indefinitely -- backpressure here stalls the whole session.
	OnOutput(sessionID string, chunk []byte) (pass []byte)
	Detach(sessionID string)
}

// SetMiddleware installs m as the sole output middleware, replacing any
// previously registered middlewares. Must be called before any session is
// created (i.e. during composition-root wiring, before wails.Run) -- it is
// not safe to mutate concurrently with live sessions.
func (s *Service) SetMiddleware(m OutputMiddleware) {
	s.middlewares = []OutputMiddleware{m}
}

// AddMiddleware appends m to the output middleware chain, run in
// registration order -- each middleware receives the previous one's pass
// output, and a middleware returning nil/empty short-circuits the rest of
// the chain (see pump.go). Same call-time constraint as SetMiddleware:
// composition-root wiring only, before any session is created.
func (s *Service) AddMiddleware(m OutputMiddleware) {
	s.middlewares = append(s.middlewares, m)
}
