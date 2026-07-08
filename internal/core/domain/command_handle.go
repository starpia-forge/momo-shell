package domain

import "fmt"

// CommandState is the execution-lifecycle state of one in-flight run_command
// invocation, driven by shell-integration Observer signals (design doc 17
// §6.4). This is the post-injection execution phase only -- the gate
// states (pending_gate/awaiting_approval/denied) are handled imperatively by
// RunCommand's blocking approval flow (C4) and are not modeled here.
type CommandState string

const (
	CmdRunning CommandState = "running"
	CmdTUI     CommandState = "tui"
	CmdDone    CommandState = "done"
)

// legalCommandTransitions: running and tui cycle into each other (alt-screen
// enter/exit), either can terminate into done. done is terminal.
var legalCommandTransitions = map[CommandState]map[CommandState]bool{
	CmdRunning: {CmdTUI: true, CmdDone: true},
	CmdTUI:     {CmdRunning: true, CmdDone: true},
	CmdDone:    {},
}

// CommandHandle is run_command's state-streaming handle (design doc 17
// §6.4/§354: "not a blocking response but a state-streaming handle,
// CommandHandle{state, exit_code?, seq}"). C4 returned only SessionID/Command
// as a documented gap; D1 fills State/ExitCode/Seq from shell-integration's
// Observer callbacks (OnCommandEnd/OnAltScreen).
type CommandHandle struct {
	SessionID string
	Command   string // the exact string injected (guarded form if guarded)

	State    CommandState
	ExitCode *int // set only once State == CmdDone (mirrors session/pump.go's ClosedPayload.ExitCode optionality)
	Seq      int  // monotonically increasing within this command's lifecycle (stream ordering/dedup); arm = 1
}

// NewCommandHandle arms a fresh command-lifecycle handle in the initial
// running state, as RunCommand does immediately after a successful
// injection.
func NewCommandHandle(sessionID, command string) *CommandHandle {
	return &CommandHandle{SessionID: sessionID, Command: command, State: CmdRunning, Seq: 1}
}

// TransitionTo moves the handle to next, rejecting illegal transitions, and
// bumps Seq on every successful transition (mirrors Delegation.TransitionTo).
func (h *CommandHandle) TransitionTo(next CommandState) error {
	if !legalCommandTransitions[h.State][next] {
		return fmt.Errorf("domain: invalid command state transition %s -> %s", h.State, next)
	}
	h.State = next
	h.Seq++
	return nil
}
