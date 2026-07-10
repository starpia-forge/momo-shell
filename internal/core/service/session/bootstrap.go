package session

// LocalBootstrapper lets an external collaborator (shellintegration)
// pre-install shell-integration hooks into a local session's process at
// spawn time, for dialects where typing hooks into the running shell and
// suppressing the echo isn't safe (ConPTY/PowerShell repaints via absolute
// cursor addressing, so suppressed rows desync the host's screen buffer
// from the frontend's -- see shellintegration/middleware.go's readiness
// gate for the dialects that still use typed injection instead). A Service
// with no bootstrapper registered behaves exactly as before this existed.
type LocalBootstrapper interface {
	// PrepareSpawn returns the extra process-launch args for shellPath's
	// dialect, or ok=false if this dialect has no spawn-time strategy (the
	// caller proceeds with no extra args; the session's own OutputMiddleware
	// Attach call falls back to typed injection as before). sessionID is
	// the id CreateLocal will use for the session about to be opened -- not
	// yet a registered session at call time.
	PrepareSpawn(sessionID, shellPath string) (args []string, ok bool)
	// DiscardSpawn releases a PrepareSpawn stash for sessionID whose Open
	// never happened (e.g. the process failed to launch).
	DiscardSpawn(sessionID string)
}

// SetLocalBootstrapper installs b as CreateLocal's spawn-time hook
// injector, replacing any previously registered bootstrapper. Must be
// called before any session is created (i.e. during composition-root
// wiring, before wails.Run) -- it is not safe to mutate concurrently with
// live sessions.
func (s *Service) SetLocalBootstrapper(b LocalBootstrapper) {
	s.localBootstrapper = b
}
