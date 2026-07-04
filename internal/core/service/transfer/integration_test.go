package transfer_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"momo-shell/internal/adapter/out/zmodem"
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
	"momo-shell/internal/core/service/session"
	"momo-shell/internal/core/service/transfer"
)

// This file proves the whole chain end to end: real rz/sz <-> pipes <->
// session.Service (pump, output middleware hook) <-> transfer.Service
// (detection, conduit) <-> the real zmodem.Engine from M4. It's the M5
// analogue of the zmodem package's own lrzsz interop tests, but exercising
// the actual production wiring (main.go's two-phase Service<->Service
// setup) instead of calling the engine directly.

// pipeStream adapts a piped external process (rz or sz) into an
// out.TerminalStream, standing in for a real SSH connection.
type pipeStream struct {
	cmd *exec.Cmd
	r   io.ReadCloser
	w   io.WriteCloser
}

func (p *pipeStream) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipeStream) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p *pipeStream) Resize(cols, rows int) error { return nil }
func (p *pipeStream) Wait() (int, error) {
	if err := p.cmd.Wait(); err != nil {
		return 1, nil
	}
	return 0, nil
}
func (p *pipeStream) Close() error {
	_ = p.w.Close()
	return p.r.Close()
}

var _ out.TerminalStream = (*pipeStream)(nil)

func spawnLrzsz(t *testing.T, dir, name string, args ...string) *pipeStream {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not installed, skipping integration test", name)
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return &pipeStream{cmd: cmd, r: stdout, w: stdin}
}

// fixedSSHOpener always returns the same pre-built stream, ignoring host/
// secret/verifier -- standing in for a real dial so the resulting session
// is domain.KindSSH (the only kind the ZMODEM middleware watches).
type fixedSSHOpener struct{ stream out.TerminalStream }

func (o *fixedSSHOpener) Open(domain.Host, string, out.HostKeyVerifier, int, int) (out.TerminalStream, error) {
	return o.stream, nil
}

type singleHostRepo struct{ host domain.Host }

func (r singleHostRepo) List() ([]domain.Host, error)            { return []domain.Host{r.host}, nil }
func (r singleHostRepo) Get(id string) (domain.Host, error)      { return r.host, nil }
func (r singleHostRepo) Save(h domain.Host) (domain.Host, error) { return h, nil }
func (r singleHostRepo) Delete(id string) error                  { return nil }
func (r singleHostRepo) TouchConnected(id string) error          { return nil }
func (r singleHostRepo) ListLabels() ([]string, error)           { return nil, nil }

type noopSecretStore struct{}

func (noopSecretStore) Set(ref string, secret []byte) error { return nil }
func (noopSecretStore) Get(ref string) ([]byte, error)      { return nil, errors.New("no secret") }
func (noopSecretStore) Delete(ref string) error             { return nil }

type noopKnownHosts struct{}

func (noopKnownHosts) Get(address string, port int, algo string) (string, bool, error) {
	return "", false, nil
}
func (noopKnownHosts) Put(address string, port int, algo, fingerprint string) error { return nil }
func (noopKnownHosts) Delete(address string, port int, algo string) error           { return nil }

// eventRecorder is a minimal out.EventPublisher for asserting what crossed
// the wire to the frontend -- in particular, that session:data never
// carries ZMODEM protocol bytes ("screen pollution").
type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	topic   string
	payload any
}

func (r *eventRecorder) Publish(topic string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{topic: topic, payload: payload})
}

func (r *eventRecorder) all() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedEvent{}, r.events...)
}

func (r *eventRecorder) sessionDataFor(id string) []byte {
	var buf []byte
	for _, e := range r.all() {
		if e.topic == "session:data:"+id {
			buf = append(buf, e.payload.([]byte)...)
		}
	}
	return buf
}

// sawZmodemEvent reports whether any transfer:zmodem:{id} event was
// published -- zmodemPayload's fields aren't exported outside the transfer
// package, so this just confirms detection fired at all; the file-content
// assertions are what actually prove the transfer worked.
func (r *eventRecorder) sawZmodemEvent(id string) bool {
	for _, e := range r.all() {
		if e.topic == "transfer:zmodem:"+id {
			return true
		}
	}
	return false
}

func newTestServices(t *testing.T, host domain.Host, stream out.TerminalStream, downloadDir string) (*session.Service, *transfer.Service, *eventRecorder) {
	t.Helper()
	pub := &eventRecorder{}
	sessionSvc := session.New(session.Deps{
		SSHOpener:  &fixedSSHOpener{stream: stream},
		HostRepo:   singleHostRepo{host: host},
		Secrets:    noopSecretStore{},
		KnownHosts: noopKnownHosts{},
		Publisher:  pub,
	})
	transferSvc := transfer.New(transfer.Deps{
		Shell:       sessionSvc,
		Pub:         pub,
		Zmodem:      zmodem.New(),
		DownloadDir: downloadDir,
	})
	sessionSvc.SetMiddleware(transferSvc)
	return sessionSvc, transferSvc, pub
}

func testHost() domain.Host {
	return domain.Host{ID: "h1", Name: "test", Address: "127.0.0.1", Port: 22, Username: "tester", AuthType: domain.AuthPassword}
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// TestIntegration_RealSzAutoDetectedAndDownloaded covers the "원격에서 sz
// file.bin 실행 → 자동 수신, 화면 오염 없음" acceptance criterion: sz's
// output flows through the real pump/middleware, is auto-detected, and the
// downloaded file lands intact -- while session:data (what the terminal
// renders) never contains ZMODEM protocol bytes.
func TestIntegration_RealSzAutoDetectedAndDownloaded(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	content := bytes.Repeat([]byte("integration test payload. "), 400)
	if err := os.WriteFile(filepath.Join(srcDir, "remote.bin"), content, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	stream := spawnLrzsz(t, srcDir, "sz", "--binary", "--quiet", "remote.bin")
	sessionSvc, _, pub := newTestServices(t, testHost(), stream, dstDir)

	info, err := sessionSvc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateSSH failed: %v", err)
	}
	defer sessionSvc.Close(info.ID)

	waitForCondition(t, 10*time.Second, func() bool {
		_, err := os.Stat(filepath.Join(dstDir, "remote.bin"))
		return err == nil
	})

	got, err := os.ReadFile(filepath.Join(dstDir, "remote.bin"))
	if err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
	if !pub.sawZmodemEvent(info.ID) {
		t.Fatalf("expected at least one transfer:zmodem:%s event", info.ID)
	}

	// Screen-pollution check: whatever reached session:data must not
	// contain a ZMODEM hex header at all -- the middleware should have
	// suppressed every protocol byte before it got there.
	rendered := pub.sessionDataFor(info.ID)
	if bytes.Contains(rendered, []byte{0x18, 0x42}) { // ZDLE 'B'
		t.Fatalf("session:data leaked ZMODEM protocol bytes: %q", rendered)
	}
}

// TestIntegration_RealRzWaitsThenReceivesOurUpload covers "원격에서 rz 실행
// → 파일 선택/드롭으로 송신 완료": rz's wait is detected, StartZmodemSend
// (standing in for the user picking files in the UI) drives our sender,
// and rz ends up with the file.
func TestIntegration_RealRzWaitsThenReceivesOurUpload(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	content := bytes.Repeat([]byte("uploaded via zmodem middleware. "), 300)
	localFile := filepath.Join(srcDir, "upload.txt")
	if err := os.WriteFile(localFile, content, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	stream := spawnLrzsz(t, dstDir, "rz", "--binary", "--quiet")
	sessionSvc, transferSvc, pub := newTestServices(t, testHost(), stream, dstDir)

	info, err := sessionSvc.CreateSSH(in.SSHOpts{HostID: "h1", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("CreateSSH failed: %v", err)
	}
	defer sessionSvc.Close(info.ID)

	if !waitForCondition(t, 5*time.Second, func() bool { return pub.sawZmodemEvent(info.ID) }) {
		t.Fatalf("expected rz's wait to be detected via transfer:zmodem:%s", info.ID)
	}

	if _, err := transferSvc.StartZmodemSend(info.ID, []string{localFile}); err != nil {
		t.Fatalf("StartZmodemSend failed: %v", err)
	}

	waitForCondition(t, 10*time.Second, func() bool {
		_, err := os.Stat(filepath.Join(dstDir, "upload.txt"))
		return err == nil
	})

	got, err := os.ReadFile(filepath.Join(dstDir, "upload.txt"))
	if err != nil {
		t.Fatalf("rz did not receive the file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}

	rendered := pub.sessionDataFor(info.ID)
	if bytes.Contains(rendered, []byte{0x18, 0x42}) {
		t.Fatalf("session:data leaked ZMODEM protocol bytes: %q", rendered)
	}
}
