package session

import (
	"errors"
	"io"
	"sync"

	"momo-terminal/internal/core/port/out"
)

var errBoom = errors.New("boom")

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
}

func (o *fakeOpener) Open(shell string, args, env []string, cwd string, cols, rows int) (out.TerminalStream, string, error) {
	if o.err != nil {
		return nil, "", o.err
	}
	return o.stream, shell, nil
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
