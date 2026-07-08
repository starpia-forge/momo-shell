package domain

// CommandHandle is run_command's return (design doc 17 §6.4's
// state-streaming handle -- fields it assigns to later work (State,
// ExitCode, Seq) are intentionally omitted here and added once D1's
// command-lifecycle FSM makes them meaningful, rather than speculatively
// stubbed now; mirrors Delegation/resolve.Verdict's incremental growth).
type CommandHandle struct {
	SessionID string
	Command   string // the exact string injected (guarded form if guarded)
}
