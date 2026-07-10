package shellintegration

import (
	"bytes"
	"sync"
	"time"

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
	// phaseWaitingReady: local session only. The hook script is built but
	// deliberately NOT written yet -- ConPTY delivers it to PowerShell
	// before PSReadLine has drawn its first prompt and is ready to read
	// input, which corrupts the injected line (dropped/reordered bytes ->
	// unterminated string -> ">>" continuation, visible to the user). This
	// phase passes output through untouched and waits for it to go idle
	// (see fireReady) before advancing to phaseWaitingSentinel and writing.
	phaseWaitingReady
	// phaseWaitingSentinel: hook script was written; watching raw output
	// for this session's own "momostart_<nonce>" text before suppressing
	// anything, so real content already in flight (e.g. an SSH banner/MOTD
	// arriving concurrently) is never swallowed. The partial line the
	// sentinel is found on is trimmed regardless (it's this injection's own
	// echoed prefix), so only complete preceding lines are ever released.
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

	// pending is the built hook script, held here while phase ==
	// phaseWaitingReady until the shell's startup output goes idle (see
	// fireReady). discardEcho, true for local sessions, drops the
	// injection's own pre-sentinel echo instead of releasing it -- once the
	// readiness gate has let the shell settle, there's no legitimate output
	// left to preserve there (contrast SSH, which never gates and must
	// still release a concurrently-arriving MOTD).
	pending     []byte
	discardEcho bool

	// readyTimer/readyDeadline debounce the phaseWaitingReady -> inject
	// transition: readyTimer is reset on every chunk received while
	// waiting, so injection fires readyDebounce after output goes quiet;
	// readyDeadline caps the total wait so pathologically chatty startup
	// output can't delay injection forever. nil/zero once fired or the
	// session is detached.
	readyTimer    *time.Timer
	readyDeadline time.Time

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
//
// If session.Service's CreateLocal consulted a LocalBootstrapper and this
// service already stashed a nonce for sessionID (via PrepareSpawn), the
// session's hooks were installed at process-spawn time -- Attach takes that
// stash and enters phaseActive directly, skipping typed injection
// entirely for that session.
func (s *Service) Attach(sessionID string, kind domain.SessionKind) {
	var dialect out.ShellDialect
	if kind == domain.KindSSH {
		dialect = out.DialectBash
	} else if shellPath, _, ok := s.shell.SessionShell(sessionID); ok {
		dialect = resolveDialect(shellPath)
	}

	st := &sessionState{dialect: dialect}
	s.mu.Lock()
	if nonce, ok := s.pendingSpawn[sessionID]; ok {
		delete(s.pendingSpawn, sessionID)
		st.nonce = nonce
		st.phase = phaseActive
	}
	s.sessions[sessionID] = st
	s.mu.Unlock()

	if st.phase == phaseActive {
		return
	}

	if dialect == out.DialectUnknown {
		return
	}

	nonce := newNonce()
	script, err := s.builder.HookScript(dialect, nonce)
	if err != nil {
		return // safe-degrade: sessionState stays at phaseNone
	}

	if kind == domain.KindLocal {
		// Don't write yet: ConPTY delivers this immediately after spawn,
		// before PowerShell/PSReadLine is ready to read input, which
		// corrupts the line. Hold it and wait for startup output to settle
		// (see fireReady).
		st.mu.Lock()
		st.nonce = nonce
		st.phase = phaseWaitingReady
		st.pending = script
		st.discardEcho = true
		st.readyDeadline = time.Now().Add(s.readyDeadline)
		st.readyTimer = time.AfterFunc(s.readyDebounce, func() { s.fireReady(sessionID, st) })
		st.mu.Unlock()
		return
	}

	st.mu.Lock()
	st.nonce = nonce
	st.phase = phaseWaitingSentinel
	st.mu.Unlock()

	// Best-effort: a WriteRaw failure leaves phaseWaitingSentinel, which
	// safely degrades to pass-through once maxSentinelWait is exceeded.
	_ = s.shell.WriteRaw(sessionID, script)
}

// fireReady is the phaseWaitingReady -> phaseWaitingSentinel transition,
// invoked by readyTimer once a local session's startup output has been
// quiet for readyDebounce (or readyDeadline is reached). The phase check
// under st.mu makes this safe against a stale/duplicate timer fire and
// against racing Detach: whichever of fireReady/Detach flips the phase
// first wins, the other is a no-op. WriteRaw is called without st.mu held.
func (s *Service) fireReady(sessionID string, st *sessionState) {
	st.mu.Lock()
	if st.phase != phaseWaitingReady {
		st.mu.Unlock()
		return
	}
	st.phase = phaseWaitingSentinel
	script := st.pending
	st.pending = nil
	st.readyTimer = nil
	st.mu.Unlock()

	_ = s.shell.WriteRaw(sessionID, script)
}

func (s *Service) Detach(sessionID string) {
	s.mu.Lock()
	st := s.sessions[sessionID]
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	if st == nil {
		return
	}

	st.mu.Lock()
	st.phase = phaseNone // any in-flight fireReady becomes a no-op
	if st.readyTimer != nil {
		st.readyTimer.Stop()
		st.readyTimer = nil
	}
	st.pending = nil
	st.mu.Unlock()
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

	case phaseWaitingReady:
		// Let startup output (banner, first prompt) render untouched, and
		// push the injection out by another readyDebounce each time more
		// arrives -- capped at readyDeadline so continuous chatty output
		// can't delay injection forever.
		if st.readyTimer != nil {
			if remaining := time.Until(st.readyDeadline); remaining > 0 {
				if remaining < s.readyDebounce {
					st.readyTimer.Reset(remaining)
				} else {
					st.readyTimer.Reset(s.readyDebounce)
				}
			}
			// remaining <= 0: deadline already reached: leave the pending
			// fire scheduled rather than pushing it out further.
		}
		return chunk, nil

	case phaseWaitingSentinel:
		sentinel := []byte("momostart_" + st.nonce)
		st.scratch = append(st.scratch, chunk...)
		idx := bytes.Index(st.scratch, sentinel)
		if idx >= 0 {
			before := append([]byte{}, st.scratch[:idx]...)
			if st.discardEcho {
				before = nil
			} else if nl := bytes.LastIndexByte(before, '\n'); nl >= 0 {
				// The partial line the sentinel sits on is always this
				// injection's own echoed prefix (e.g. bash's "printf '")
				// plus a pre-injection prompt fragment that gets redrawn
				// once hooks are confirmed -- trim it, keeping only
				// complete preceding lines (e.g. a concurrently-arriving
				// SSH MOTD).
				before = before[:nl+1]
			} else {
				before = nil
			}
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
