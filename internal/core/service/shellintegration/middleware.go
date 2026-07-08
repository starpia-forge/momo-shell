package shellintegration

import (
	"bytes"
	"sync"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// bootstrapPhase tracks how far a session's injection has progressed.
// phaseNone is the zero value: a session this service didn't (or couldn't)
// inject into behaves as pure pass-through for its whole lifetime.
type bootstrapPhase int

const (
	// phaseNone: dialect unknown, or hook install was never attempted --
	// OnOutput is a no-op pass-through. Terminal state (Reinject is the
	// only way out, and only for a session with a known dialect).
	phaseNone bootstrapPhase = iota
	// phaseWaitingSentinel: hook script was written; watching raw output
	// for this session's own "momostart_<nonce>" text before suppressing
	// anything, so real content already in flight (e.g. an SSH banner/MOTD
	// arriving concurrently) is never swallowed.
	phaseWaitingSentinel
	// phaseSuppressing: sentinel seen; every byte is discarded (not
	// rendered, no events dispatched) until the matching hookinstalled
	// marker confirms the hooks are live.
	phaseSuppressing
	// phaseActive: hooks confirmed installed; normal OSC parsing, marker
	// stripping, and observer notification.
	phaseActive
)

// maxSentinelWait bounds how much raw output this service buffers while
// waiting for its own injection to echo back before giving up and
// degrading to pass-through -- protects against unbounded memory growth if
// WriteRaw silently failed or the dialect assumption (SSH) was wrong.
const maxSentinelWait = 8192

type sessionState struct {
	mu sync.Mutex

	dialect out.ShellDialect
	phase   bootstrapPhase
	nonce   string

	scratch []byte // accumulates raw output while phase == phaseWaitingSentinel

	osc         parser
	altScreen   altScreenDetector
	atPrompt    bool
	inAltScreen bool
}

// Attach resolves the session's shell dialect and, if supported, injects
// the hook-installation script. kind is authoritative for SSH (assume
// bash -- doc 17 §12's "미지원→승인 강등" principle covers a wrong guess:
// hooks simply never confirm and the session stays at phaseWaitingSentinel
// until maxSentinelWait, then degrades to pass-through). For local
// sessions, the resolved shell path (via ShellInjector.SessionShell) picks
// the dialect.
func (s *Service) Attach(sessionID string, kind domain.SessionKind) {
	var dialect out.ShellDialect
	if kind == domain.KindSSH {
		dialect = out.DialectBash
	} else if shellPath, _, ok := s.shell.SessionShell(sessionID); ok {
		dialect = resolveDialect(shellPath)
	}

	st := &sessionState{dialect: dialect}
	s.mu.Lock()
	s.sessions[sessionID] = st
	s.mu.Unlock()

	if dialect == out.DialectUnknown {
		return
	}

	nonce := newNonce()
	script, err := s.builder.HookScript(dialect, nonce)
	if err != nil {
		return // safe-degrade: sessionState stays at phaseNone
	}

	st.mu.Lock()
	st.nonce = nonce
	st.phase = phaseWaitingSentinel
	st.mu.Unlock()

	// Best-effort: a WriteRaw failure leaves phaseWaitingSentinel, which
	// safely degrades to pass-through once maxSentinelWait is exceeded.
	_ = s.shell.WriteRaw(sessionID, script)
}

func (s *Service) Detach(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// OnOutput never holds a lock while notifying observers -- events are
// collected under sessionState's lock, then dispatched after it's released,
// so an observer callback can safely call back into this service (e.g.
// AtPrompt) without deadlocking.
func (s *Service) OnOutput(sessionID string, chunk []byte) []byte {
	st := s.get(sessionID)
	if st == nil {
		return chunk
	}

	st.mu.Lock()
	rendered, notes := s.stepLocked(st, chunk)
	st.mu.Unlock()

	for _, n := range notes {
		s.dispatch(sessionID, n)
	}
	return rendered
}

type notification struct {
	kind      eventKind
	exitCode  int
	altScreen bool
	entered   bool
	nonce     string
	payload   string
}

const evAltScreen eventKind = -1 // out of parser's eventKind range; middleware-only

// stepLocked runs one chunk through the bootstrap/parsing state machine.
// Must be called with st.mu held.
func (s *Service) stepLocked(st *sessionState, chunk []byte) (rendered []byte, notes []notification) {
	switch st.phase {
	case phaseNone:
		return chunk, nil

	case phaseWaitingSentinel:
		sentinel := []byte("momostart_" + st.nonce)
		st.scratch = append(st.scratch, chunk...)
		idx := bytes.Index(st.scratch, sentinel)
		if idx >= 0 {
			before := append([]byte{}, st.scratch[:idx]...)
			rest := st.scratch[idx:]
			st.scratch = nil
			st.phase = phaseSuppressing
			consumeSuppressed(st, rest)
			return before, nil
		}
		if len(st.scratch) > maxSentinelWait {
			flushed := st.scratch
			st.scratch = nil
			st.phase = phaseNone
			return flushed, nil
		}
		return nil, nil // buffered; nothing released until sentinel or cap

	case phaseSuppressing:
		consumeSuppressed(st, chunk)
		return nil, nil

	case phaseActive:
		rendered, events := st.osc.feed(chunk)
		for _, ev := range events {
			switch ev.kind {
			case evPromptStart:
				st.atPrompt = true
				notes = append(notes, notification{kind: evPromptStart})
			case evCommandStart:
				st.atPrompt = false
				notes = append(notes, notification{kind: evCommandStart})
			case evCommandDone:
				notes = append(notes, notification{kind: evCommandDone, exitCode: ev.exitCode})
			case evHookInstalled:
				// Reinject landed while already active, or a stale/
				// duplicate marker -- already active, nothing to do.
			case evProbeReply:
				notes = append(notes, notification{kind: evProbeReply, nonce: ev.nonce, payload: ev.payload})
			}
		}
		enter, exit := st.altScreen.scan(rendered)
		if enter && !st.inAltScreen {
			st.inAltScreen = true
			notes = append(notes, notification{kind: evAltScreen, altScreen: true, entered: true})
		}
		if exit && st.inAltScreen {
			st.inAltScreen = false
			notes = append(notes, notification{kind: evAltScreen, altScreen: true, entered: false})
		}
		return rendered, notes
	}
	return chunk, nil
}

// consumeSuppressed parses data while suppressing it (phaseSuppressing),
// flipping st.phase to phaseActive once the matching hookinstalled marker
// is found. Must be called with st.mu held. Any content in data AFTER the
// hookinstalled marker within the same call (rare -- the residual prompt
// cycle PROMPT_COMMAND/prompt fires immediately following hook install can
// land in the same read chunk) is intentionally dropped as bootstrap noise
// rather than precisely split; the shell's next real command cycle
// re-establishes AtPrompt correctly.
func consumeSuppressed(st *sessionState, data []byte) {
	_, events := st.osc.feed(data)
	for _, ev := range events {
		if ev.kind == evHookInstalled && ev.nonce == st.nonce {
			st.phase = phaseActive
			return
		}
	}
}

func (s *Service) dispatch(sessionID string, n notification) {
	if n.kind == evProbeReply {
		// Routed to a waiting Query call, not fanned out to observers --
		// this is a request/response reply, not a lifecycle event.
		s.routeProbe(n.nonce, n.payload)
		return
	}
	for _, o := range s.observers {
		switch n.kind {
		case evPromptStart:
			o.OnPrompt(sessionID)
		case evCommandStart:
			o.OnCommandStart(sessionID)
		case evCommandDone:
			o.OnCommandEnd(sessionID, n.exitCode)
		case evAltScreen:
			o.OnAltScreen(sessionID, n.entered)
		}
	}
}
