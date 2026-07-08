package aicontrol

import (
	"errors"
	"sync"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/service/aicontrol/resolve"
)

// fakeHostRepo is a hand-rolled in-memory out.HostRepository (house style:
// no mock framework -- see share/mocks_test.go's fakeHostRepo).
type fakeHostRepo struct {
	mu    sync.Mutex
	hosts map[string]domain.Host
}

func newFakeHostRepo(hosts ...domain.Host) *fakeHostRepo {
	r := &fakeHostRepo{hosts: make(map[string]domain.Host)}
	for _, h := range hosts {
		r.hosts[h.ID] = h
	}
	return r
}

func (r *fakeHostRepo) List() ([]domain.Host, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := make([]domain.Host, 0, len(r.hosts))
	for _, h := range r.hosts {
		list = append(list, h)
	}
	return list, nil
}

func (r *fakeHostRepo) Get(id string) (domain.Host, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hosts[id]
	if !ok {
		return domain.Host{}, errNotFound
	}
	return h, nil
}

func (r *fakeHostRepo) Save(h domain.Host) (domain.Host, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hosts[h.ID] = h
	return h, nil
}

func (r *fakeHostRepo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hosts, id)
	return nil
}

func (r *fakeHostRepo) TouchConnected(id string) error { return nil }

func (r *fakeHostRepo) ListLabels() ([]string, error) { return nil, nil }

var errNotFound = errors.New("aicontrol test: not found")

// fakeSessionCreator is a hand-rolled SessionCreator. createFunc lets a test
// script CreateSSH's outcome (success/error) per case; sessionShellFunc does
// the same for SessionShell's existence check, defaulting to "exists" since
// most tests (ConnectHost) never call it.
type fakeSessionCreator struct {
	createFunc       func(opts in.SSHOpts) (domain.SessionInfo, error)
	sessionShellFunc func(sessionID string) (shell string, kind domain.SessionKind, ok bool)
	writeFunc        func(sessionID string, data []byte) error

	mu     sync.Mutex
	writes []fakeWrite
}

type fakeWrite struct {
	sessionID string
	data      []byte
}

func (f *fakeSessionCreator) CreateSSH(opts in.SSHOpts) (domain.SessionInfo, error) {
	return f.createFunc(opts)
}

func (f *fakeSessionCreator) SessionShell(sessionID string) (shell string, kind domain.SessionKind, ok bool) {
	if f.sessionShellFunc != nil {
		return f.sessionShellFunc(sessionID)
	}
	return "", "", true
}

func (f *fakeSessionCreator) Write(sessionID string, data []byte) error {
	f.mu.Lock()
	f.writes = append(f.writes, fakeWrite{sessionID: sessionID, data: append([]byte(nil), data...)})
	f.mu.Unlock()
	if f.writeFunc != nil {
		return f.writeFunc(sessionID, data)
	}
	return nil
}

func (f *fakeSessionCreator) allWrites() []fakeWrite {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeWrite, len(f.writes))
	copy(out, f.writes)
	return out
}

// fakeResolver is a hand-rolled CommandResolver returning a canned Verdict
// per test case (house style: no mock framework).
type fakeResolver struct {
	verdict resolve.Verdict
}

func (f *fakeResolver) Resolve(command, dialect string) resolve.Verdict {
	return f.verdict
}

// recordingPublisher is a hand-rolled out.EventPublisher that records every
// Publish call (share/mocks_test.go mirror).
type recordingPublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	topic   string
	payload any
}

func (p *recordingPublisher) Publish(topic string, payload any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, recordedEvent{topic: topic, payload: payload})
}

func (p *recordingPublisher) all() []recordedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]recordedEvent, len(p.events))
	copy(out, p.events)
	return out
}
