package share

import (
	"context"
	"errors"
	"sync"

	"momo-shell/internal/core/domain"
)

var errNotFound = errors.New("share: not found")

// fakeHostRepo is a minimal in-memory out.HostRepository for tests --
// share.Service only ever calls Get.
type fakeHostRepo struct {
	mu    sync.Mutex
	hosts map[string]domain.Host
}

func newFakeHostRepo() *fakeHostRepo {
	return &fakeHostRepo{hosts: make(map[string]domain.Host)}
}

func (r *fakeHostRepo) List() ([]domain.Host, error) { return nil, nil }

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
func (r *fakeHostRepo) ListLabels() ([]string, error)  { return nil, nil }

// fakeClients is an in-memory out.ShareClientRepository for tests.
type fakeClients struct {
	mu      sync.Mutex
	clients map[string]domain.ShareClient
	tokens  map[string]string // clientID -> tokenHash
}

func newFakeClients() *fakeClients {
	return &fakeClients{clients: make(map[string]domain.ShareClient), tokens: make(map[string]string)}
}

func (c *fakeClients) List() ([]domain.ShareClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]domain.ShareClient, 0, len(c.clients))
	for _, client := range c.clients {
		out = append(out, client)
	}
	return out, nil
}

func (c *fakeClients) FindByTokenHash(hash string) (domain.ShareClient, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, h := range c.tokens {
		if h == hash {
			return c.clients[id], true, nil
		}
	}
	return domain.ShareClient{}, false, nil
}

func (c *fakeClients) Save(client domain.ShareClient, tokenHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clients[client.ID] = client
	c.tokens[client.ID] = tokenHash
	return nil
}

func (c *fakeClients) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.clients, id)
	delete(c.tokens, id)
	return nil
}

func (c *fakeClients) TouchSeen(id string) error { return nil }

func (c *fakeClients) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.clients)
}

// fakeSettings is an in-memory out.ShareSettings for tests.
type fakeSettings struct {
	mu            sync.Mutex
	instanceID    string
	sharedHostIDs []string
}

func newFakeSettings() *fakeSettings {
	return &fakeSettings{}
}

func (s *fakeSettings) InstanceID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.instanceID == "" {
		s.instanceID = "fake-instance-id"
	}
	return s.instanceID, nil
}

func (s *fakeSettings) SharedHostIDs() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.sharedHostIDs...), nil
}

func (s *fakeSettings) SetSharedHostIDs(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sharedHostIDs = append([]string{}, ids...)
	return nil
}

// fakeServer is an in-memory out.ShareServer for tests -- no real network.
type fakeServer struct {
	mu        sync.Mutex
	started   int
	stopped   int
	startErr  error
	startPort int
}

func (f *fakeServer) Start() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started++
	if f.startErr != nil {
		return 0, f.startErr
	}
	return f.startPort, nil
}

func (f *fakeServer) Stop(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped++
	return nil
}

func (f *fakeServer) startCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func (f *fakeServer) stopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

// recordingPublisher records every Publish call for assertions.
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
