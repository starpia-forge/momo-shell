package history

import (
	"sync"
	"time"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/in"
	"momo-terminal/internal/core/port/out"
)

// commitQueueSize bounds how many committed lines can be queued for
// persistence before OnInput starts dropping them rather than blocking the
// session's Write() hot path.
const commitQueueSize = 256

// HistoryAppendedPayload is published on history:appended.
type HistoryAppendedPayload struct {
	ID         int64   `json:"id"`
	HostID     *string `json:"hostId,omitempty"`
	Command    string  `json:"command"`
	ExecutedAt int64   `json:"executedAt"`
}

type commitJob struct {
	hostID  *string
	command string
	at      int64
}

type sessionRecorder struct {
	hostID *string

	mu       sync.Mutex
	recorder *lineRecorder
}

// Service implements in.HistoryUseCase (browsing/managing history) and
// session.CommandTap (capturing it), since both need the same per-session
// recorder state. Persistence happens on a single writer goroutine fed by a
// buffered channel, so a burst of commits never blocks a session's input path.
type Service struct {
	repo out.HistoryRepository
	pub  out.EventPublisher

	mu        sync.Mutex
	recorders map[string]*sessionRecorder

	jobs chan commitJob
	done chan struct{}
	wg   sync.WaitGroup
}

var _ in.HistoryUseCase = (*Service)(nil)

func New(repo out.HistoryRepository, pub out.EventPublisher) *Service {
	s := &Service{
		repo:      repo,
		pub:       pub,
		recorders: make(map[string]*sessionRecorder),
		jobs:      make(chan commitJob, commitQueueSize),
		done:      make(chan struct{}),
	}
	s.wg.Add(1)
	go s.writeLoop()
	return s
}

// Attach registers a newly-running session for capture. hostID == "" means local.
func (s *Service) Attach(sessionID string, hostID string) {
	var hp *string
	if hostID != "" {
		hp = &hostID
	}
	s.mu.Lock()
	s.recorders[sessionID] = &sessionRecorder{hostID: hp, recorder: newLineRecorder()}
	s.mu.Unlock()
}

func (s *Service) Detach(sessionID string) {
	s.mu.Lock()
	delete(s.recorders, sessionID)
	s.mu.Unlock()
}

func (s *Service) OnInput(sessionID string, data []byte) {
	rec := s.recorderFor(sessionID)
	if rec == nil {
		return
	}

	rec.mu.Lock()
	lines := rec.recorder.feed(data)
	rec.mu.Unlock()

	now := time.Now().Unix()
	for _, line := range lines {
		select {
		case s.jobs <- commitJob{hostID: rec.hostID, command: line, at: now}:
		default:
			// Writer goroutine is backed up -- drop rather than block input.
		}
	}
}

func (s *Service) OnOutput(sessionID string, data []byte) {
	rec := s.recorderFor(sessionID)
	if rec == nil {
		return
	}
	rec.mu.Lock()
	rec.recorder.scanAltScreen(data)
	rec.mu.Unlock()
}

func (s *Service) recorderFor(sessionID string) *sessionRecorder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recorders[sessionID]
}

// Close drains any queued commits and stops the writer goroutine. Call once
// during app shutdown, before the database is closed.
func (s *Service) Close() {
	close(s.done)
	s.wg.Wait()
}

func (s *Service) writeLoop() {
	defer s.wg.Done()
	for {
		select {
		case job := <-s.jobs:
			s.persist(job)
		case <-s.done:
			for {
				select {
				case job := <-s.jobs:
					s.persist(job)
				default:
					return
				}
			}
		}
	}
}

// persist saves one committed line, coalescing a consecutive repeat of the
// same command (per host) into a timestamp bump instead of a new row.
func (s *Service) persist(job commitJob) {
	if shouldMask(job.command) {
		return
	}

	if last, found, err := s.repo.LastForHost(job.hostID); err == nil && found && last.Command == job.command {
		if err := s.repo.TouchLast(last.ID, job.at); err == nil {
			s.publish(domain.HistoryEntry{ID: last.ID, HostID: job.hostID, Command: job.command, ExecutedAt: job.at})
		}
		return
	}

	entry, err := s.repo.Append(job.hostID, job.command, job.at)
	if err != nil {
		return
	}
	s.publish(entry)
}

func (s *Service) publish(e domain.HistoryEntry) {
	s.pub.Publish(out.TopicHistoryAppended(), HistoryAppendedPayload{
		ID:         e.ID,
		HostID:     e.HostID,
		Command:    e.Command,
		ExecutedAt: e.ExecutedAt,
	})
}

func (s *Service) List(q domain.HistoryQuery) ([]domain.HistoryEntry, error) {
	return s.repo.List(q)
}

func (s *Service) Delete(id int64) error {
	return s.repo.Delete(id)
}

func (s *Service) Clear(hostID *string) error {
	return s.repo.Clear(hostID)
}
