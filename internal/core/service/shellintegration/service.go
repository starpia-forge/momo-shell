// Package shellintegration implements session.OutputMiddleware: it
// runtime-injects OSC133 prompt/command/exit hooks into a session's shell
// (no rc/profile edits -- see internal/adapter/out/shellintegration for the
// verified per-dialect scripts), strips the resulting OSC markers from the
// stream, and notifies registered Observers of prompt/command/alt-screen
// lifecycle events. It does not know or care who consumes those events --
// see doc 17 §6.2/§6.4 for how resolve/aicontrol build on this seam.
package shellintegration

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// ErrSessionNotFound is returned by Reinject/AtPrompt-adjacent operations
// on a session this service was never Attach-ed to (or that has since
// Detach-ed).
var ErrSessionNotFound = errors.New("shellintegration: session not found")

// ShellInjector is the narrow, core-defined interface this service needs
// from the session service -- consumer-side, matching how session.CommandTap
// and transfer.ShellAccess are defined by their consumers rather than by
// session itself.
type ShellInjector interface {
	// WriteRaw injects hook-installation bytes directly, bypassing command-
	// history capture and the input-blocked guard (session.Service.WriteRaw).
	WriteRaw(sessionID string, data []byte) error
	// SessionShell reports a session's resolved shell path and kind
	// (session.Service.SessionShell). shell is empty for SSH sessions --
	// the remote shell is never recorded, so SSH dialect is an assumption
	// (see resolveDialect), not a lookup.
	SessionShell(sessionID string) (shell string, kind domain.SessionKind, ok bool)
}

// Observer is notified of shell-integration lifecycle events once a
// session's hooks are confirmed installed. Methods are called from the
// session's pump goroutine (via OutputMiddleware.OnOutput, with no
// service-internal locks held) and must not block.
type Observer interface {
	OnPrompt(sessionID string)
	OnCommandStart(sessionID string)
	OnCommandEnd(sessionID string, exitCode int)
	OnAltScreen(sessionID string, entered bool)
}

// Deps are the out-ports/collaborators Service needs.
type Deps struct {
	Shell   ShellInjector
	Builder out.ShellScriptBuilder
}

// Service implements session.OutputMiddleware (see middleware.go) to
// runtime-inject and parse shell-integration OSC markers, structurally --
// it does not import the session package, matching transfer.Service's
// relationship to session.Service.
type Service struct {
	shell   ShellInjector
	builder out.ShellScriptBuilder

	mu       sync.Mutex
	sessions map[string]*sessionState

	// observers is append-only, composition-root wiring (see AddObserver) --
	// read without a lock from the pump goroutine, matching
	// session.Service's own taps/middlewares slices.
	observers []Observer

	// probeMu guards probes, which correlates an in-flight Query call to the
	// pump goroutine's evProbeReply delivery by nonce (see probe.go). Never
	// held at the same time as a sessionState's own mu -- Query and
	// routeProbe each only ever touch probeMu, so the two can't deadlock
	// against each other or against OnOutput's own locking.
	probeMu sync.Mutex
	probes  map[string]chan string
}

func New(deps Deps) *Service {
	return &Service{
		shell:    deps.Shell,
		builder:  deps.Builder,
		sessions: make(map[string]*sessionState),
		probes:   make(map[string]chan string),
	}
}

// AddObserver registers o to be notified of shell-integration lifecycle
// events. Must be called before any session is created (composition-root
// wiring only), matching session.Service.AddTap/AddMiddleware -- not
// concurrency-safe to mutate once sessions are live.
func (s *Service) AddObserver(o Observer) {
	s.observers = append(s.observers, o)
}

// AtPrompt reports whether a session's shell is currently sitting at a
// fresh prompt (hooks installed, no command running) -- the gate a probe
// injector must check before writing anything into the shell (doc 17 §6.2:
// "프롬프트 상태에서만 주입"). False for an unknown session, a session whose
// hooks never installed (unsupported dialect, injection failed), or one
// mid-command.
func (s *Service) AtPrompt(sessionID string) bool {
	st := s.get(sessionID)
	if st == nil {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.phase == phaseActive && st.atPrompt
}

// Reinject re-runs the bootstrap injection sequence with a fresh nonce --
// for a session whose hooks were lost (su/exec/nested ssh into a shell
// that never got the runtime injection). Returns ErrSessionNotFound if this
// service was never Attach-ed to sessionID, or an error if the session's
// dialect is unknown (nothing to reinject).
func (s *Service) Reinject(sessionID string) error {
	st := s.get(sessionID)
	if st == nil {
		return ErrSessionNotFound
	}

	st.mu.Lock()
	dialect := st.dialect
	st.mu.Unlock()
	if dialect == out.DialectUnknown {
		return errors.New("shellintegration: dialect unknown for this session, nothing to reinject")
	}

	nonce := newNonce()
	script, err := s.builder.HookScript(dialect, nonce)
	if err != nil {
		return err
	}

	st.mu.Lock()
	st.nonce = nonce
	st.phase = phaseWaitingSentinel
	st.scratch = nil
	st.atPrompt = false
	st.inAltScreen = false
	st.osc = parser{}
	st.altScreen = altScreenDetector{}
	st.mu.Unlock()

	return s.shell.WriteRaw(sessionID, script)
}

func (s *Service) get(sessionID string) *sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID]
}

// resolveDialect maps a local session's resolved shell path to the dialect
// whose hook script to inject. Anything unrecognized degrades to
// DialectUnknown -- callers must skip injection rather than guess.
func resolveDialect(shellPath string) out.ShellDialect {
	lower := strings.ToLower(shellPath)
	switch {
	case strings.Contains(lower, "bash"):
		return out.DialectBash
	case strings.Contains(lower, "powershell"), strings.Contains(lower, "pwsh"):
		return out.DialectPowerShell
	default:
		return out.DialectUnknown
	}
}

func newNonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
