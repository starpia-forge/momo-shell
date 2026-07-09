package aicontrol

import (
	"context"
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
	snapshotFunc     func() []domain.Session
	closeFunc        func(sessionID string) error

	mu     sync.Mutex
	writes []fakeWrite
	closes []string
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

func (f *fakeSessionCreator) Snapshot() []domain.Session {
	if f.snapshotFunc != nil {
		return f.snapshotFunc()
	}
	return nil
}

func (f *fakeSessionCreator) Close(sessionID string) error {
	f.mu.Lock()
	f.closes = append(f.closes, sessionID)
	f.mu.Unlock()
	if f.closeFunc != nil {
		return f.closeFunc(sessionID)
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

func (f *fakeSessionCreator) allCloses() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.closes))
	copy(out, f.closes)
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

// fakeScrollbackReader is a hand-rolled ScrollbackReader returning a canned
// (data, nextSeq) pair per test case, and recording the (sessionID, sinceSeq)
// it was called with so tests can assert ReadScrollback's fan-out.
type fakeScrollbackReader struct {
	data []byte
	next uint64

	called      bool
	gotSession  string
	gotSinceSeq uint64
}

func (f *fakeScrollbackReader) ReadSince(sessionID string, sinceSeq uint64) ([]byte, uint64) {
	f.called = true
	f.gotSession = sessionID
	f.gotSinceSeq = sinceSeq
	return f.data, f.next
}

// fakeOutputMasker is a hand-rolled OutputMasker returning a canned
// (masked, gated, notice) result, recording the (sessionID, chunk) it was
// called with.
type fakeOutputMasker struct {
	masked []byte
	gated  bool
	notice string

	called     bool
	gotSession string
	gotChunk   []byte
}

func (f *fakeOutputMasker) Apply(sessionID string, chunk []byte) ([]byte, bool, string) {
	f.called = true
	f.gotSession = sessionID
	f.gotChunk = append([]byte(nil), chunk...)
	return f.masked, f.gated, f.notice
}

// fakeShellStateReader is a hand-rolled ShellStateReader (state.go, B4)
// delegating to a func field so each test supplies its own canned
// vars/error, mirroring fakeSessionCreator's func-field style.
type fakeShellStateReader struct {
	readVarsFunc func(ctx context.Context, sessionID string, names []string) (map[string]string, error)
}

func (f *fakeShellStateReader) ReadVars(ctx context.Context, sessionID string, names []string) (map[string]string, error) {
	return f.readVarsFunc(ctx, sessionID, names)
}

// fakeMCPClients is an in-memory out.MCPClientRepository for tests
// (share/mocks_test.go's fakeClients mirror, over domain.MCPClient). It
// supports seeding rows with a preset tokenHash (including Revoked:true
// rows, since FindByTokenHash must return those as-is per the port's
// contract) and records TouchSeen calls so AuthClient tests can assert it
// fired on success.
type fakeMCPClients struct {
	mu      sync.Mutex
	clients map[string]domain.MCPClient
	tokens  map[string]string // clientID -> tokenHash

	touchedSeen []string
}

func newFakeMCPClients() *fakeMCPClients {
	return &fakeMCPClients{clients: make(map[string]domain.MCPClient), tokens: make(map[string]string)}
}

// seed registers a client under a known tokenHash, bypassing Save, so tests
// can construct rows (e.g. Revoked:true) that HandlePair's issueToken path
// would never produce.
func (c *fakeMCPClients) seed(client domain.MCPClient, tokenHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clients[client.ClientID] = client
	c.tokens[client.ClientID] = tokenHash
}

func (c *fakeMCPClients) List() ([]domain.MCPClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]domain.MCPClient, 0, len(c.clients))
	for _, client := range c.clients {
		out = append(out, client)
	}
	return out, nil
}

func (c *fakeMCPClients) FindByTokenHash(hash string) (domain.MCPClient, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, h := range c.tokens {
		if h == hash {
			return c.clients[id], true, nil
		}
	}
	return domain.MCPClient{}, false, nil
}

func (c *fakeMCPClients) Save(client domain.MCPClient, tokenHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clients[client.ClientID] = client
	c.tokens[client.ClientID] = tokenHash
	return nil
}

func (c *fakeMCPClients) Revoke(clientID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	client, ok := c.clients[clientID]
	if !ok {
		return errNotFound
	}
	client.Revoked = true
	c.clients[clientID] = client
	return nil
}

func (c *fakeMCPClients) Delete(clientID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.clients, clientID)
	delete(c.tokens, clientID)
	return nil
}

func (c *fakeMCPClients) TouchSeen(clientID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.touchedSeen = append(c.touchedSeen, clientID)
	return nil
}

func (c *fakeMCPClients) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.clients)
}

func (c *fakeMCPClients) wasTouched(clientID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range c.touchedSeen {
		if id == clientID {
			return true
		}
	}
	return false
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
