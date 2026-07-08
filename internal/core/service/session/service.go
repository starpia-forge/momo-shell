package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// ErrSessionNotFound is returned by operations on an unknown or already-closed session ID.
var ErrSessionNotFound = errors.New("session: not found")

// ErrSessionConnecting is returned by Write/Resize on an SSH session that
// hasn't finished connecting yet.
var ErrSessionConnecting = errors.New("session: still connecting")

// closeAllGrace bounds how long CloseAll waits for every session's pump to
// finish draining before giving up (used during app shutdown).
const closeAllGrace = 3 * time.Second

// Deps are the out-ports Service needs. SSHOpener/HostRepo/Secrets/KnownHosts
// are only used by CreateSSH; a Service used purely for local sessions may
// leave them nil.
type Deps struct {
	LocalOpener out.LocalTerminalOpener
	SSHOpener   out.SSHTerminalOpener
	HostRepo    out.HostRepository
	Secrets     out.SecretStore
	KnownHosts  out.KnownHostsRepository
	Publisher   out.EventPublisher
	// Tap observes input/output for command-history capture. Nil disables it.
	Tap CommandTap
}

// Service implements in.SessionUseCase. It owns session lifecycle and
// delegates transport to whatever TerminalStream-producing adapters it's
// given -- local PTY and SSH are treated identically once a stream exists.
type Service struct {
	mu       sync.Mutex
	sessions map[string]*liveSession

	localOpener out.LocalTerminalOpener
	sshOpener   out.SSHTerminalOpener
	hostRepo    out.HostRepository
	secrets     out.SecretStore
	knownHosts  out.KnownHostsRepository
	pub         out.EventPublisher
	taps        []CommandTap
	middlewares []OutputMiddleware
	wg          sync.WaitGroup
}

// liveSession tracks one session's runtime state. For a local session,
// stream is set at construction and never changes. For an SSH session,
// stream starts nil (Connecting) and is set once by connectSSH; mu guards
// that single handoff since Write/Resize/Close may run concurrently with it.
type liveSession struct {
	session *domain.Session
	readCh  chan []byte

	mu     sync.Mutex
	stream out.TerminalStream
	// fs caches the SFTP subsystem opened via FileSystem, so repeated file
	// browser calls reuse one channel instead of opening a new one each time.
	fs out.RemoteFileSystem

	// hostKeyResp and cancelCh are only used while an SSH session is
	// Connecting; nil for local sessions.
	hostKeyResp chan string
	cancelCh    chan struct{}
	cancelOnce  sync.Once

	// inputBlocked drops keystrokes while a ZMODEM transfer owns the shell
	// channel, so stray typing can't corrupt the protocol stream. The
	// frontend's own overlay is the primary defense; this is the backend
	// backstop.
	inputBlocked atomic.Bool
}

func (live *liveSession) setStream(stream out.TerminalStream) {
	live.mu.Lock()
	live.stream = stream
	live.mu.Unlock()
}

func (live *liveSession) getStream() out.TerminalStream {
	live.mu.Lock()
	defer live.mu.Unlock()
	return live.stream
}

// fileSystem lazily opens and caches the stream's SFTP subsystem. Returns an
// error if the stream doesn't implement out.FileSystemCapable (local
// sessions) or the subsystem can't be opened (e.g. SFTP disabled server-side).
func (live *liveSession) fileSystem() (out.RemoteFileSystem, error) {
	live.mu.Lock()
	defer live.mu.Unlock()
	if live.fs != nil {
		return live.fs, nil
	}
	capable, ok := live.stream.(out.FileSystemCapable)
	if !ok {
		return nil, errors.New("session: file transfer not supported for this session")
	}
	fs, err := capable.OpenFileSystem()
	if err != nil {
		return nil, err
	}
	live.fs = fs
	return fs, nil
}

// closeFileSystem releases the cached SFTP subsystem, if any. Called from
// the pump's shutdown paths before the underlying stream closes.
func (live *liveSession) closeFileSystem() {
	live.mu.Lock()
	fs := live.fs
	live.fs = nil
	live.mu.Unlock()
	if fs != nil {
		_ = fs.Close()
	}
}

var _ in.SessionUseCase = (*Service)(nil)

func New(deps Deps) *Service {
	s := &Service{
		sessions:    make(map[string]*liveSession),
		localOpener: deps.LocalOpener,
		sshOpener:   deps.SSHOpener,
		hostRepo:    deps.HostRepo,
		secrets:     deps.Secrets,
		knownHosts:  deps.KnownHosts,
		pub:         deps.Publisher,
	}
	if deps.Tap != nil {
		s.taps = append(s.taps, deps.Tap)
	}
	return s
}

func (s *Service) CreateLocal(opts in.LocalOpts) (domain.SessionInfo, error) {
	stream, resolvedShell, err := s.localOpener.Open(opts.Shell, nil, opts.Env, opts.Cwd, opts.Cols, opts.Rows)
	if err != nil {
		return domain.SessionInfo{}, err
	}

	id := uuid.NewString()
	sess := domain.NewSession(id, domain.KindLocal, resolvedShell, opts.Cols, opts.Rows)
	_ = sess.TransitionTo(domain.StateRunning)

	live := &liveSession{
		session: sess,
		stream:  stream,
		readCh:  make(chan []byte, 64),
	}

	s.mu.Lock()
	s.sessions[id] = live
	s.mu.Unlock()

	for _, t := range s.taps {
		t.Attach(id, "")
	}
	for _, m := range s.middlewares {
		m.Attach(id, domain.KindLocal)
	}

	s.wg.Add(2)
	go s.readLoop(live)
	go s.pump(id, live)

	s.pub.Publish(out.TopicSessionState(id), StatePayload{State: string(domain.StateRunning)})

	return sess.Info(), nil
}

func (s *Service) Write(id string, data []byte) error {
	live, ok := s.get(id)
	if !ok {
		return ErrSessionNotFound
	}
	if live.inputBlocked.Load() {
		return nil
	}
	stream := live.getStream()
	if stream == nil {
		return ErrSessionConnecting
	}
	_, err := stream.Write(data)
	if err == nil {
		for _, t := range s.taps {
			t.OnInput(id, data)
		}
	}
	return err
}

// WriteRaw writes directly to the stream, bypassing CommandTap.OnInput and
// the input-blocked guard -- for ZMODEM protocol bytes, which must never
// pollute command-history capture and must flow even while user input is
// blocked (the transfer service is the one blocking it).
func (s *Service) WriteRaw(id string, data []byte) error {
	live, ok := s.get(id)
	if !ok {
		return ErrSessionNotFound
	}
	stream := live.getStream()
	if stream == nil {
		return ErrSessionConnecting
	}
	_, err := stream.Write(data)
	return err
}

// SetInputBlocked drops (Write returns nil without writing) or resumes
// keystrokes for a session -- used by the transfer service while a ZMODEM
// exchange owns the shell channel.
func (s *Service) SetInputBlocked(id string, blocked bool) error {
	live, ok := s.get(id)
	if !ok {
		return ErrSessionNotFound
	}
	live.inputBlocked.Store(blocked)
	return nil
}

// FileSystem lazily opens (and caches, per session) the SFTP subsystem on
// the session's connection. See in.SessionUseCase.FileSystem.
func (s *Service) FileSystem(id string) (out.RemoteFileSystem, error) {
	live, ok := s.get(id)
	if !ok {
		return nil, ErrSessionNotFound
	}
	if live.getStream() == nil {
		return nil, ErrSessionConnecting
	}
	return live.fileSystem()
}

// SessionShell reports a session's resolved shell and kind, for observers
// that need to pick a dialect-specific behavior (e.g. shell-integration hook
// selection). shell is the resolved local shell path and is empty for SSH
// sessions -- the remote shell is never recorded (see domain.Session.Shell).
func (s *Service) SessionShell(id string) (shell string, kind domain.SessionKind, ok bool) {
	live, found := s.get(id)
	if !found {
		return "", "", false
	}
	return live.session.Shell, live.session.Kind, true
}

func (s *Service) Resize(id string, cols, rows int) error {
	live, ok := s.get(id)
	if !ok {
		return ErrSessionNotFound
	}
	stream := live.getStream()
	if stream == nil {
		return ErrSessionConnecting
	}
	return stream.Resize(cols, rows)
}

// Close closes a running session's stream, or -- for an SSH session still
// Connecting -- cancels the in-flight connect/host-key wait and removes the
// session immediately (there's no stream or pump goroutine yet to drain).
func (s *Service) Close(id string) error {
	live, ok := s.get(id)
	if !ok {
		return ErrSessionNotFound
	}
	stream := live.getStream()
	if stream == nil {
		live.cancelOnce.Do(func() { close(live.cancelCh) })
		s.remove(id)
		return nil
	}
	return stream.Close()
}

// CloseAll synchronously closes every live session, waiting up to
// closeAllGrace for their pumps to finish. Used on app shutdown.
func (s *Service) CloseAll() {
	s.mu.Lock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	s.mu.Unlock()

	for _, id := range ids {
		_ = s.Close(id)
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(closeAllGrace):
	}
}

func (s *Service) get(id string) (*liveSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, ok := s.sessions[id]
	return live, ok
}

func (s *Service) remove(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}
