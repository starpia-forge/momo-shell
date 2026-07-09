package domain

// ShellState is a point-in-time snapshot of a delegated session's shell
// state -- cwd and a fixed set of "main" env vars (design doc 17 §5.1,
// FR-3): "위임 중 세션의 cwd·주요 env를 이벤트로 프론트에 노출". Queried
// on demand (GetShellState), not tracked continuously.
type ShellState struct {
	SessionID string
	Cwd       string            // $PWD probe result; "" if unresolvable
	Env       map[string]string // currently-set vars only -- unset vars are omitted, not zero-valued
}
