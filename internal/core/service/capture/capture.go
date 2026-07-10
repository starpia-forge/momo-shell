// Package capture implements E3-b's per-command original-output capture: a
// session.CommandTap (structurally, no import -- scrollback.Service's
// pattern) that isolates one command's raw bytes from the shared scrollback
// ring and seals them to that command's audit row once its lifecycle ends.
//
// This is deliberately NOT built on top of scrollback.ReadSince. Session
// output flows through session/pump.go as: for each chunk, every
// OutputMiddleware runs first (shellintegration strips OSC-133 markers and
// fires its Observer -- aicontrol.OnCommandEnd -- synchronously as part of
// that pass), THEN every CommandTap runs (scrollback's ring append happens
// here). So the chunk that carries the command-end marker reaches capture's
// OnOutput strictly AFTER aicontrol.OnCommandEnd has already fired for it.
// A design that reacts to OnCommandEnd by spawning a goroutine to read
// scrollback immediately races that tap-phase append -- for a short command
// whose entire output and end-marker land in one PTY read (the common
// case), the read can observe nothing at all.
//
// capture avoids the race structurally: End() only flags the in-flight
// capture as ending; the actual seal happens inside OnOutput, AFTER that
// call's bytes are appended, and OnOutput runs (as a tap) only after the
// same pump iteration's End()-triggering middleware call has already
// returned. The trailing chunk is therefore always included by construction
// -- not by timing luck. See per-method docs for the remaining edge cases
// (superseding an unsealed capture, a fully-suppressed end chunk, session
// teardown).
package capture

import (
	"fmt"
	"sync"
)

// defaultCapBytes bounds retained bytes per captured command (head-first --
// once full, further bytes are dropped and a truncation marker is appended
// on seal). Matches scrollback's ring capacity; most AI-driven command
// output is well under this.
const defaultCapBytes = 256 * 1024

// OutputSink is the narrow, consumer-defined interface Service needs to
// persist a sealed capture -- audit.Service.AttachOutput satisfies it
// structurally (ScrollbackReader/OutputMasker's pattern in aicontrol).
type OutputSink interface {
	AttachOutput(auditID int64, output []byte) error
}

// Service captures each in-flight command's output in isolation, keyed by
// sessionID (mirroring scrollback.Service's per-session buffers, but scoped
// to a command's lifetime rather than the whole session).
type Service struct {
	sink     OutputSink
	capBytes int

	mu    sync.Mutex
	state map[string]*captureState // sessionID -> in-flight capture, absent once sealed
}

type captureState struct {
	auditID   int64
	buf       []byte
	truncated bool
	ending    bool
}

func New(sink OutputSink) *Service {
	return NewWithCapacity(sink, defaultCapBytes)
}

func NewWithCapacity(sink OutputSink, capBytes int) *Service {
	return &Service{sink: sink, capBytes: capBytes, state: make(map[string]*captureState)}
}

// Begin arms a fresh capture for sessionID linked to auditID -- called from
// aicontrol.RunCommand at the same point it arms the command's
// CommandHandle. The one-in-flight-per-session invariant isn't strictly
// enforced there (a second RunCommand can arm before the first's
// OnCommandEnd lands), so if a prior capture for this session is still
// open, it is sealed to its OWN auditID first -- output is never cross-
// linked to the wrong command.
func (s *Service) Begin(sessionID string, auditID int64) {
	s.mu.Lock()
	prior := s.state[sessionID]
	s.state[sessionID] = &captureState{auditID: auditID}
	s.mu.Unlock()

	if prior != nil {
		s.seal(prior)
	}
}

// End marks sessionID's in-flight capture as finished. It does not seal
// immediately: see the package doc for why the actual seal is deferred to
// the next OnOutput call, which (for the OnCommandEnd-driven case) is
// guaranteed to be the same pump iteration's already-inbound chunk.
func (s *Service) End(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.state[sessionID]; ok {
		st.ending = true
	}
}

// Attach registers a newly-running session (session.CommandTap's contract).
// A no-op beyond what Begin/OnOutput already handle -- unlike scrollback's
// always-on ring, capture only tracks sessions with an actual in-flight
// command.
func (s *Service) Attach(sessionID string, hostID string) {}

// OnInput is a no-op: capture only needs command output (scrollback.
// Service.OnInput's rationale -- the PTY echoes input back as output).
func (s *Service) OnInput(sessionID string, data []byte) {}

// OnOutput appends data to sessionID's in-flight capture, head-capped at
// capBytes (further bytes are dropped once full, keeping the earliest,
// most forensically relevant portion -- the command and its initial
// output/errors -- rather than the tail). If End already flagged this
// capture as ending, the seal happens here, after the append: taps run
// after middlewares within one pump iteration, so the bytes that triggered
// OnCommandEnd are already appended by the time ending is observed true.
func (s *Service) OnOutput(sessionID string, data []byte) {
	s.mu.Lock()
	st, ok := s.state[sessionID]
	if !ok {
		s.mu.Unlock()
		return
	}
	if room := s.capBytes - len(st.buf); room > 0 {
		n := room
		if n > len(data) {
			n = len(data)
		}
		st.buf = append(st.buf, data[:n]...)
		if n < len(data) {
			st.truncated = true
		}
	} else if len(data) > 0 {
		st.truncated = true
	}
	ending := st.ending
	if ending {
		delete(s.state, sessionID)
	}
	s.mu.Unlock()

	if ending {
		s.seal(st)
	}
}

// Detach seals any still-open capture for sessionID (session.CommandTap's
// contract -- the session closed mid-command). This is the catch-all for
// every case that doesn't otherwise produce a further OnOutput call to
// trigger a deferred seal (e.g. BackgroundCommand's End() with no more
// session output before the session eventually closes).
func (s *Service) Detach(sessionID string) {
	s.mu.Lock()
	st, ok := s.state[sessionID]
	if ok {
		delete(s.state, sessionID)
	}
	s.mu.Unlock()

	if ok {
		s.seal(st)
	}
}

// seal hands st's buffered bytes to the sink from a new goroutine, off the
// caller's own -- OnOutput/Detach run on the session's pump goroutine, and
// shellintegration's Observer contract (which drives End, transitively)
// requires callbacks not to block.
func (s *Service) seal(st *captureState) {
	auditID, buf := st.auditID, st.buf
	if st.truncated {
		buf = append(append([]byte(nil), buf...), []byte(fmt.Sprintf("\n...[output truncated at %d bytes]", s.capBytes))...)
	}
	go func() {
		_ = s.sink.AttachOutput(auditID, buf)
	}()
}
