package share

import (
	"context"
	"errors"
	"sync"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
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
	deviceName    string
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

func (s *fakeSettings) DeviceName() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deviceName, nil
}

func (s *fakeSettings) SetDeviceName(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceName = name
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

// fakeSecretStore is an in-memory out.SecretStore for tests.
type fakeSecretStore struct {
	mu      sync.Mutex
	secrets map[string][]byte
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{secrets: make(map[string][]byte)}
}

func (s *fakeSecretStore) Set(ref string, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[ref] = secret
	return nil
}

func (s *fakeSecretStore) Get(ref string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.secrets[ref]
	if !ok {
		return nil, errNotFound
	}
	return v, nil
}

func (s *fakeSecretStore) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, ref)
	return nil
}

func (s *fakeSecretStore) has(ref string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.secrets[ref]
	return ok
}

// fakePeers is an in-memory out.PeerRepository for tests.
type fakePeers struct {
	mu    sync.Mutex
	peers map[string]domain.Peer
}

func newFakePeers() *fakePeers {
	return &fakePeers{peers: make(map[string]domain.Peer)}
}

func (p *fakePeers) List() ([]domain.Peer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]domain.Peer, 0, len(p.peers))
	for _, peer := range p.peers {
		out = append(out, peer)
	}
	return out, nil
}

func (p *fakePeers) Get(id string) (domain.Peer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	peer, ok := p.peers[id]
	if !ok {
		return domain.Peer{}, errNotFound
	}
	return peer, nil
}

func (p *fakePeers) Save(peer domain.Peer) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers[peer.ID] = peer
	return nil
}

func (p *fakePeers) Delete(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.peers, id)
	return nil
}

func (p *fakePeers) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.peers)
}

// fakeAnnouncer is an in-memory out.PeerAnnouncer for tests -- no real mDNS.
type fakeAnnouncer struct {
	mu           sync.Mutex
	announceErr  error
	announced    int
	stopped      int
	lastInstance string
	lastName     string
	lastPort     int
}

func (a *fakeAnnouncer) Announce(instanceID, name string, port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.announced++
	a.lastInstance, a.lastName, a.lastPort = instanceID, name, port
	return a.announceErr
}

func (a *fakeAnnouncer) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopped++
}

func (a *fakeAnnouncer) stopCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopped
}

// fakeBrowser is an in-memory out.PeerBrowser for tests -- no real mDNS.
// Tests drive discovery by calling push() directly.
type fakeBrowser struct {
	mu       sync.Mutex
	onUpdate func([]out.DiscoveredPeer)
	stopped  int
}

func (b *fakeBrowser) Start(onUpdate func([]out.DiscoveredPeer)) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onUpdate = onUpdate
	return nil
}

func (b *fakeBrowser) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped++
}

func (b *fakeBrowser) push(peers []out.DiscoveredPeer) {
	b.mu.Lock()
	onUpdate := b.onUpdate
	b.mu.Unlock()
	if onUpdate != nil {
		onUpdate(peers)
	}
}

// fakePeerClient is an in-memory out.PeerClient for tests -- no real network.
type fakePeerClient struct {
	mu         sync.Mutex
	infoFunc   func(address string, port int, certFP string) (out.PeerInfo, string, error)
	pairFunc   func(address string, port int, certFP, pin, clientName string) (string, string, error)
	hostsFunc  func(address string, port int, certFP, token string) ([]domain.SharedHost, error)
	hostsCalls int
}

func (c *fakePeerClient) Info(address string, port int, certFP string) (out.PeerInfo, string, error) {
	return c.infoFunc(address, port, certFP)
}

func (c *fakePeerClient) Pair(address string, port int, certFP, pin, clientName string) (string, string, error) {
	return c.pairFunc(address, port, certFP, pin, clientName)
}

func (c *fakePeerClient) FetchHosts(address string, port int, certFP, token string) ([]domain.SharedHost, error) {
	c.mu.Lock()
	c.hostsCalls++
	c.mu.Unlock()
	return c.hostsFunc(address, port, certFP, token)
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

// testService bundles a Service under test with every fake collaborator,
// so tests can reach whichever ones they need without a long positional
// return tuple.
type testService struct {
	svc        *Service
	hostRepo   *fakeHostRepo
	clients    *fakeClients
	settings   *fakeSettings
	server     *fakeServer
	pub        *recordingPublisher
	secrets    *fakeSecretStore
	peers      *fakePeers
	announcer  *fakeAnnouncer
	browser    *fakeBrowser
	peerClient *fakePeerClient
}

func newTestService() *testService {
	ts := &testService{
		hostRepo:  newFakeHostRepo(),
		clients:   newFakeClients(),
		settings:  newFakeSettings(),
		server:    &fakeServer{startPort: 47800},
		pub:       &recordingPublisher{},
		secrets:   newFakeSecretStore(),
		peers:     newFakePeers(),
		announcer: &fakeAnnouncer{},
		browser:   &fakeBrowser{},
		peerClient: &fakePeerClient{
			infoFunc:  func(string, int, string) (out.PeerInfo, string, error) { return out.PeerInfo{}, "", errNotFound },
			pairFunc:  func(string, int, string, string, string) (string, string, error) { return "", "", errNotFound },
			hostsFunc: func(string, int, string, string) ([]domain.SharedHost, error) { return nil, errNotFound },
		},
	}
	ts.svc = New(Deps{
		HostRepo:   ts.hostRepo,
		Clients:    ts.clients,
		Settings:   ts.settings,
		Pub:        ts.pub,
		Peers:      ts.peers,
		Secrets:    ts.secrets,
		Announcer:  ts.announcer,
		Browser:    ts.browser,
		PeerClient: ts.peerClient,
	})
	ts.svc.SetServer(ts.server)
	return ts
}
