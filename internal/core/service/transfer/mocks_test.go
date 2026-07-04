package transfer

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// fakeRemoteFS is an out.RemoteFileSystem backed by a real temp directory,
// so ReadDir/Stat/Open/Create exercise real disk I/O without a real SFTP
// server -- the interface is the thing under test, not the transport.
type fakeRemoteFS struct {
	root string
}

func newFakeRemoteFS(t *testing.T) *fakeRemoteFS {
	return &fakeRemoteFS{root: t.TempDir()}
}

// native maps a forward-slash "remote" path (matching real SFTP semantics)
// onto this fake's temp directory using the host OS's path conventions.
func (f *fakeRemoteFS) native(p string) string {
	return filepath.Join(f.root, filepath.FromSlash(p))
}

func (f *fakeRemoteFS) ReadDir(p string) ([]domain.RemoteEntry, error) {
	entries, err := os.ReadDir(f.native(p))
	if err != nil {
		return nil, err
	}
	result := make([]domain.RemoteEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, domain.RemoteEntry{
			Name:     e.Name(),
			Path:     path.Join(p, e.Name()),
			Size:     info.Size(),
			Mode:     uint32(info.Mode()),
			ModeText: info.Mode().String(),
			ModTime:  info.ModTime(),
			IsDir:    e.IsDir(),
		})
	}
	return result, nil
}

func (f *fakeRemoteFS) Stat(p string) (domain.RemoteEntry, error) {
	info, err := os.Stat(f.native(p))
	if err != nil {
		return domain.RemoteEntry{}, err
	}
	return domain.RemoteEntry{
		Name: info.Name(), Path: p, Size: info.Size(), Mode: uint32(info.Mode()),
		ModeText: info.Mode().String(), ModTime: info.ModTime(), IsDir: info.IsDir(),
	}, nil
}

func (f *fakeRemoteFS) Open(p string) (io.ReadCloser, error)    { return os.Open(f.native(p)) }
func (f *fakeRemoteFS) Create(p string) (io.WriteCloser, error) { return os.Create(f.native(p)) }
func (f *fakeRemoteFS) Mkdir(p string) error                    { return os.Mkdir(f.native(p), 0o755) }

func (f *fakeRemoteFS) Rename(oldPath, newPath string) error {
	return os.Rename(f.native(oldPath), f.native(newPath))
}

func (f *fakeRemoteFS) Remove(p string) error {
	info, err := os.Stat(f.native(p))
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.Remove(f.native(p))
	}
	return os.Remove(f.native(p))
}

func (f *fakeRemoteFS) Chmod(p string, mode uint32) error {
	return os.Chmod(f.native(p), os.FileMode(mode))
}

func (f *fakeRemoteFS) HomeDir() (string, error) { return "/", nil }
func (f *fakeRemoteFS) Close() error             { return nil }

var _ out.RemoteFileSystem = (*fakeRemoteFS)(nil)

// gatedFS wraps a RemoteFileSystem so every Create's first Write blocks
// until gate is closed, and counts how many Creates were reached -- lets
// tests assert exactly how many tasks are actively running versus queued.
type gatedFS struct {
	out.RemoteFileSystem
	gate chan struct{}

	mu      sync.Mutex
	started int
}

func (g *gatedFS) Create(p string) (io.WriteCloser, error) {
	wc, err := g.RemoteFileSystem.Create(p)
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	g.started++
	g.mu.Unlock()
	return &blockingWriteCloser{WriteCloser: wc, gate: g.gate}, nil
}

func (g *gatedFS) startedCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.started
}

// blockingWriteCloser blocks the first Write until gate is closed.
type blockingWriteCloser struct {
	io.WriteCloser
	gate chan struct{}
	once sync.Once
}

func (w *blockingWriteCloser) Write(p []byte) (int, error) {
	w.once.Do(func() { <-w.gate })
	return w.WriteCloser.Write(p)
}

var errNoFileSystem = errors.New("shell: no file system for session")

// fakeShellAccess is a ShellAccess stub mapping session IDs to pre-built
// RemoteFileSystems (or errors), letting tests control per-session SFTP
// availability without a real session service or SSH connection.
type fakeShellAccess struct {
	mu   sync.Mutex
	fs   map[string]out.RemoteFileSystem
	errs map[string]error
}

func newFakeShellAccess() *fakeShellAccess {
	return &fakeShellAccess{fs: make(map[string]out.RemoteFileSystem), errs: make(map[string]error)}
}

func (s *fakeShellAccess) set(sessionID string, fs out.RemoteFileSystem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fs[sessionID] = fs
}

func (s *fakeShellAccess) setErr(sessionID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs[sessionID] = err
}

func (s *fakeShellAccess) FileSystem(sessionID string) (out.RemoteFileSystem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.errs[sessionID]; ok {
		return nil, err
	}
	fs, ok := s.fs[sessionID]
	if !ok {
		return nil, errNoFileSystem
	}
	return fs, nil
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
