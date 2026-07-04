package session

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/in"
	"momo-terminal/internal/core/port/out"
)

// ErrSessionNotFound is returned by operations on an unknown or already-closed session ID.
var ErrSessionNotFound = errors.New("session: not found")

// closeAllGrace bounds how long CloseAll waits for every session's pump to
// finish draining before giving up (used during app shutdown).
const closeAllGrace = 3 * time.Second

// Service implements in.SessionUseCase. It owns session lifecycle and
// delegates transport to whatever out.LocalTerminalOpener it's given.
type Service struct {
	mu       sync.Mutex
	sessions map[string]*liveSession

	opener out.LocalTerminalOpener
	pub    out.EventPublisher
	wg     sync.WaitGroup
}

type liveSession struct {
	session *domain.Session
	stream  out.TerminalStream
	readCh  chan []byte
}

var _ in.SessionUseCase = (*Service)(nil)

func New(opener out.LocalTerminalOpener, pub out.EventPublisher) *Service {
	return &Service{
		sessions: make(map[string]*liveSession),
		opener:   opener,
		pub:      pub,
	}
}

func (s *Service) CreateLocal(opts in.LocalOpts) (domain.SessionInfo, error) {
	stream, err := s.opener.Open(opts.Shell, nil, opts.Env, opts.Cwd, opts.Cols, opts.Rows)
	if err != nil {
		return domain.SessionInfo{}, err
	}

	id := uuid.NewString()
	sess := domain.NewSession(id, domain.KindLocal, opts.Shell, opts.Cols, opts.Rows)
	_ = sess.TransitionTo(domain.StateRunning)

	live := &liveSession{
		session: sess,
		stream:  stream,
		readCh:  make(chan []byte, 64),
	}

	s.mu.Lock()
	s.sessions[id] = live
	s.mu.Unlock()

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
	_, err := live.stream.Write(data)
	return err
}

func (s *Service) Resize(id string, cols, rows int) error {
	live, ok := s.get(id)
	if !ok {
		return ErrSessionNotFound
	}
	return live.stream.Resize(cols, rows)
}

func (s *Service) Close(id string) error {
	live, ok := s.get(id)
	if !ok {
		return ErrSessionNotFound
	}
	return live.stream.Close()
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
