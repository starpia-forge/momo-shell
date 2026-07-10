package aicontrol

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// This file implements shellintegration.Observer (D1, doc 18) -- driving the
// execution-lifecycle half of CommandHandle (running/tui/done) from F3's
// OSC-133-derived callbacks. Production registration (AddObserver at the
// composition root) is A7's job, same deferral as the rest of aicontrol's
// wiring; these callbacks are exercised directly in tests until then.

// commandStatePayload is TopicMCPCommandState's payload, defined next to its
// publisher (house convention -- cf. commandApprovalPayload).
type commandStatePayload struct {
	SessionID string `json:"sessionId"`
	Seq       int    `json:"seq"`
	State     string `json:"state"`
	ExitCode  *int   `json:"exitCode,omitempty"`
}

func (s *Service) publishCommandState(h domain.CommandHandle) {
	if s.pub == nil {
		return
	}
	s.pub.Publish(out.TopicMCPCommandState(), commandStatePayload{
		SessionID: h.SessionID,
		Seq:       h.Seq,
		State:     string(h.State),
		ExitCode:  h.ExitCode,
	})
}

// OnCommandEnd transitions the session's in-flight handle (if any) to done
// with its exit code. Sessions with no armed handle, or one already done, are
// ignored -- e.g. a command the user typed directly rather than via
// RunCommand.
func (s *Service) OnCommandEnd(sessionID string, exitCode int) {
	s.mu.Lock()
	h, ok := s.commands[sessionID]
	if !ok || h.State.IsTerminal() {
		s.mu.Unlock()
		return
	}
	code := exitCode
	h.ExitCode = &code
	_ = h.TransitionTo(domain.CmdDone) // running/tui -> done are both legal
	snapshot := *h
	s.mu.Unlock()

	if s.capture != nil {
		// E3-b: flag the in-flight capture as finished. This does not seal
		// it yet -- the seal happens on the next OnOutput call, which (for
		// the chunk that triggered this very callback) is guaranteed to
		// follow within the same pump iteration. See capture package doc.
		s.capture.End(sessionID)
	}
	s.publishCommandState(snapshot)
}

// OnAltScreen transitions the session's in-flight handle between running and
// tui as the shell enters/exits the alternate screen buffer.
func (s *Service) OnAltScreen(sessionID string, entered bool) {
	s.mu.Lock()
	h, ok := s.commands[sessionID]
	if !ok || h.State.IsTerminal() {
		s.mu.Unlock()
		return
	}
	next := domain.CmdRunning
	if entered {
		next = domain.CmdTUI
	}
	if h.State == next { // already there -- ignore a duplicate signal
		s.mu.Unlock()
		return
	}
	if err := h.TransitionTo(next); err != nil {
		s.mu.Unlock()
		return
	}
	snapshot := *h
	s.mu.Unlock()
	s.publishCommandState(snapshot)
}

// OnCommandStart and OnPrompt complete shellintegration.Observer's method set.
// D1's execution FSM doesn't need OnCommandStart (RunCommand already arms
// CmdRunning at injection time -- see run.go); OnPrompt is C2's probe-timing
// signal (Service.AtPrompt), not D1's concern. D3's echo-off secret-input
// detection (once expected to also hang off OnPrompt) dissolved without
// shipping -- echo-off is unobservable over SSH/ConPTY (doc 22) -- so
// OnPrompt stays a pure probe-timing hook with no secret-input consumer.
func (s *Service) OnCommandStart(sessionID string) {}
func (s *Service) OnPrompt(sessionID string)       {}
