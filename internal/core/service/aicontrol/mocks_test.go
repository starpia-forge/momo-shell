package aicontrol

import (
	"errors"
	"sync"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
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
// script CreateSSH's outcome (success/error) per case.
type fakeSessionCreator struct {
	createFunc func(opts in.SSHOpts) (domain.SessionInfo, error)
}

func (f *fakeSessionCreator) CreateSSH(opts in.SSHOpts) (domain.SessionInfo, error) {
	return f.createFunc(opts)
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
