package session

// CommandTap lets an external observer (the history service) watch a
// session's raw input/output bytes for the length of its lifetime, without
// this package knowing anything about how -- or whether -- that capture
// works. A Service with a nil Tap behaves exactly as before this existed.
type CommandTap interface {
	// Attach registers a newly-running session. hostID is "" for local sessions.
	Attach(sessionID string, hostID string)
	OnInput(sessionID string, data []byte)
	OnOutput(sessionID string, data []byte)
	Detach(sessionID string)
}
