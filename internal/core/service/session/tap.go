package session

// CommandTap lets an external observer (the history service) watch a
// session's raw input/output bytes for the length of its lifetime, without
// this package knowing anything about how -- or whether -- that capture
// works. A Service with no taps registered behaves exactly as before this
// existed.
type CommandTap interface {
	// Attach registers a newly-running session. hostID is "" for local sessions.
	Attach(sessionID string, hostID string)
	OnInput(sessionID string, data []byte)
	OnOutput(sessionID string, data []byte)
	Detach(sessionID string)
}

// AddTap registers an additional CommandTap, fanned out to alongside
// Deps.Tap (if any) in registration order. Must be called before any
// session is created (i.e. during composition-root wiring, before
// wails.Run) -- it is not safe to mutate concurrently with live sessions.
func (s *Service) AddTap(t CommandTap) {
	s.taps = append(s.taps, t)
}
