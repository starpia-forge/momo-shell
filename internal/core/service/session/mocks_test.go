package session

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

var errBoom = errors.New("boom")

// fakeMiddleware records every hook call and applies a caller-supplied
// transform to each output chunk (returning nil suppresses it), so tests
// can exercise the passthrough and suppression paths through pump.
type fakeMiddleware struct {
	mu         sync.Mutex
	attached   []string
	detached   []string
	seenOutput [][]byte
	transform  func(sessionID string, chunk []byte) []byte
}

func (m *fakeMiddleware) Attach(sessionID string, kind domain.SessionKind) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attached = append(m.attached, sessionID)
}

func (m *fakeMiddleware) OnOutput(sessionID string, chunk []byte) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(chunk))
	copy(cp, chunk)
	m.seenOutput = append(m.seenOutput, cp)
	if m.transform == nil {
		return chunk
	}
	return m.transform(sessionID, chunk)
}

func (m *fakeMiddleware) Detach(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.detached = append(m.detached, sessionID)
}

func (m *fakeMiddleware) attachedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.attached))
	copy(out, m.attached)
	return out
}

func (m *fakeMiddleware) detachedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.detached))
	copy(out, m.detached)
	return out
}

func (m *fakeMiddleware) outputCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.seenOutput)
}

// fakeStream is a fully in-memory out.TerminalStream for tests.
type fakeStream struct {
	mu      sync.Mutex
	writes  [][]byte
	resizes [][2]int

	toRead    chan []byte
	pending   []byte // leftover from a chunk larger than the last Read buffer; Read-goroutine-exclusive
	closeCh   chan struct{}
	closeOnce sync.Once
	exitCode  int
}

func newFakeStream() *fakeStream {
	return &fakeStream{
		toRead:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
	}
}

// Read honors the io.Reader contract that a single call may read fewer
// bytes than len(p): anything left over from a chunk bigger than the
// caller's buffer is held in pending for the next call.
func (f *fakeStream) Read(p []byte) (int, error) {
	if len(f.pending) > 0 {
		n := copy(p, f.pending)
		f.pending = f.pending[n:]
		return n, nil
	}

	chunk, ok := <-f.toRead
	if !ok {
		return 0, io.EOF
	}
	n := copy(p, chunk)
	if n < len(chunk) {
		f.pending = chunk[n:]
	}
	return n, nil
}

func (f *fakeStream) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(p))
	copy(cp, p)
	f.writes = append(f.writes, cp)
	return len(p), nil
}

func (f *fakeStream) Resize(cols, rows int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizes = append(f.resizes, [2]int{cols, rows})
	return nil
}

// Close models an explicit close request (e.g. the user closing a tab).
// The reported exit code is arbitrary (0) since a forced close doesn't
// necessarily correspond to a real process exit code.
func (f *fakeStream) Close() error {
	f.closeWithExit(0)
	return nil
}

// simulateProcessExit models the shell exiting on its own, with a real exit
// code, independent of anyone calling Close().
func (f *fakeStream) simulateProcessExit(code int) {
	f.closeWithExit(code)
}

func (f *fakeStream) closeWithExit(code int) {
	f.closeOnce.Do(func() {
		f.exitCode = code
		close(f.toRead)
		close(f.closeCh)
	})
}

func (f *fakeStream) Wait() (int, error) {
	<-f.closeCh
	return f.exitCode, nil
}

func (f *fakeStream) push(data []byte) {
	f.toRead <- data
}

func (f *fakeStream) writtenBytes() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.writes))
	copy(out, f.writes)
	return out
}

func (f *fakeStream) resizeCalls() [][2]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][2]int, len(f.resizes))
	copy(out, f.resizes)
	return out
}

// fakeOpener always hands back a pre-built fakeStream so tests can hold a
// reference to it for pushing data / asserting writes.
type fakeOpener struct {
	stream *fakeStream
	err    error

	mu       sync.Mutex
	lastArgs []string // records the args Open was last called with
}

func (o *fakeOpener) Open(shell string, args, env []string, cwd string, cols, rows int) (out.TerminalStream, string, error) {
	o.mu.Lock()
	o.lastArgs = append([]string{}, args...)
	o.mu.Unlock()
	if o.err != nil {
		return nil, "", o.err
	}
	return o.stream, shell, nil
}

func (o *fakeOpener) Resolve(shell string) string { return shell }

func (o *fakeOpener) getLastArgs() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.lastArgs
}

// openerFunc hands back a fresh stream (by calling next) on every Open call,
// for tests that create multiple sessions and need a distinct fake per one.
type openerFunc func() (*fakeStream, error)

func (f openerFunc) Open(shell string, args, env []string, cwd string, cols, rows int) (out.TerminalStream, string, error) {
	s, err := f()
	if err != nil {
		return nil, "", err
	}
	return s, shell, nil
}

func (f openerFunc) Resolve(shell string) string { return shell }

// fakeBootstrapper records PrepareSpawn/DiscardSpawn calls and hands back a
// caller-supplied args slice (or ok=false if none was configured), so tests
// can assert both that CreateLocal consults it and that its returned args
// reach Open.
type fakeBootstrapper struct {
	mu   sync.Mutex
	args []string // returned by PrepareSpawn when ok is true
	ok   bool

	prepared []string // sessionIDs PrepareSpawn was called with
	shells   []string // shellPaths PrepareSpawn was called with
	discards []string // sessionIDs DiscardSpawn was called with
}

func (b *fakeBootstrapper) PrepareSpawn(sessionID, shellPath string) ([]string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.prepared = append(b.prepared, sessionID)
	b.shells = append(b.shells, shellPath)
	if !b.ok {
		return nil, false
	}
	return b.args, true
}

func (b *fakeBootstrapper) DiscardSpawn(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.discards = append(b.discards, sessionID)
}

func (b *fakeBootstrapper) snapshot() (prepared, shells, discards []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string{}, b.prepared...), append([]string{}, b.shells...), append([]string{}, b.discards...)
}

// recordingPublisher records every Publish call and can be told to panic
// exactly once, on the first topic matching panicOnPrefix, to exercise the
// pump's recover path.
type recordingPublisher struct {
	mu            sync.Mutex
	events        []recordedEvent
	panicOnPrefix string
	panicked      bool
}

type recordedEvent struct {
	topic   string
	payload any
}

func (p *recordingPublisher) Publish(topic string, payload any) {
	p.mu.Lock()
	shouldPanic := p.panicOnPrefix != "" && !p.panicked && hasPrefix(topic, p.panicOnPrefix)
	if shouldPanic {
		p.panicked = true
	}
	p.events = append(p.events, recordedEvent{topic: topic, payload: payload})
	p.mu.Unlock()

	if shouldPanic {
		panic("publish boom")
	}
}

func (p *recordingPublisher) all() []recordedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]recordedEvent, len(p.events))
	copy(out, p.events)
	return out
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

var errHostNotFound = errors.New("host: not found")

// fakeHostRepo is an in-memory out.HostRepository for SSH session tests.
type fakeHostRepo struct {
	mu      sync.Mutex
	hosts   map[string]domain.Host
	touched []string
}

func newFakeHostRepo(hosts ...domain.Host) *fakeHostRepo {
	m := make(map[string]domain.Host, len(hosts))
	for _, h := range hosts {
		m[h.ID] = h
	}
	return &fakeHostRepo{hosts: m}
}

func (r *fakeHostRepo) List() ([]domain.Host, error) { return nil, nil }

func (r *fakeHostRepo) Get(id string) (domain.Host, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hosts[id]
	if !ok {
		return domain.Host{}, errHostNotFound
	}
	return h, nil
}

func (r *fakeHostRepo) Save(h domain.Host) (domain.Host, error) { return h, nil }
func (r *fakeHostRepo) Delete(id string) error                  { return nil }

func (r *fakeHostRepo) TouchConnected(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.touched = append(r.touched, id)
	return nil
}

func (r *fakeHostRepo) ListLabels() ([]string, error) { return nil, nil }

func (r *fakeHostRepo) touchedIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.touched))
	copy(out, r.touched)
	return out
}

// fakeSecretStore is an in-memory out.SecretStore for SSH session tests.
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
		return nil, errors.New("secret: not found")
	}
	return v, nil
}

func (s *fakeSecretStore) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, ref)
	return nil
}

// fakeKnownHostsRepo is an in-memory out.KnownHostsRepository for SSH
// session tests.
type fakeKnownHostsRepo struct {
	mu      sync.Mutex
	entries map[string]string
}

func newFakeKnownHostsRepo() *fakeKnownHostsRepo {
	return &fakeKnownHostsRepo{entries: make(map[string]string)}
}

func knownHostsKey(address string, port int, algo string) string {
	return fmt.Sprintf("%s:%d:%s", address, port, algo)
}

func (r *fakeKnownHostsRepo) Get(address string, port int, algo string) (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fp, ok := r.entries[knownHostsKey(address, port, algo)]
	return fp, ok, nil
}

func (r *fakeKnownHostsRepo) Put(address string, port int, algo string, fingerprint string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[knownHostsKey(address, port, algo)] = fingerprint
	return nil
}

func (r *fakeKnownHostsRepo) Delete(address string, port int, algo string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, knownHostsKey(address, port, algo))
	return nil
}

// fakeSSHOpener is an out.SSHTerminalOpener stub. If probeAlgo is set, Open
// invokes the verifier with (probeAlgo, probeFingerprint) before deciding
// what to return, letting tests exercise the real host-key round trip
// (including CreateSSH's Connecting-state prompt/response wiring) without a
// real network connection.
type fakeSSHOpener struct {
	stream *fakeStream
	err    error

	probeAlgo        string
	probeFingerprint string

	mu         sync.Mutex
	lastSecret string
}

func (o *fakeSSHOpener) Open(host domain.Host, secret string, verifier out.HostKeyVerifier, cols, rows int) (out.TerminalStream, error) {
	o.mu.Lock()
	o.lastSecret = secret
	o.mu.Unlock()

	if o.probeAlgo != "" {
		if _, err := verifier(o.probeAlgo, o.probeFingerprint); err != nil {
			return nil, err
		}
	}
	if o.err != nil {
		return nil, o.err
	}
	return o.stream, nil
}

func (o *fakeSSHOpener) secretSeen() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.lastSecret
}
