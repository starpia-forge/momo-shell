package zmodem

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// cmdConn adapts an external process's stdin/stdout into an io.ReadWriter,
// the same shape our conduit will be in production (a shell channel).
type cmdConn struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (c *cmdConn) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *cmdConn) Write(p []byte) (int, error) { return c.w.Write(p) }
func (c *cmdConn) Close() error {
	c.w.Close()
	return c.r.Close()
}

func spawnPiped(t *testing.T, dir, name string, args ...string) (*cmdConn, *exec.Cmd) {
	t.Helper()
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
	return &cmdConn{r: stdout, w: stdin}, cmd
}

func requireBinary(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not installed, skipping real lrzsz interop test", name)
	}
	return path
}

// TestLrzszInterop_SendToRealRz proves our Send() actually speaks ZMODEM
// correctly to a real receiver, not just to our own Receive(): rz is the
// ground truth our hand-rolled framing/CRC/escaping choices are checked
// against.
func TestLrzszInterop_SendToRealRz(t *testing.T) {
	rzPath := requireBinary(t, "rz")

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	content := bytes.Repeat([]byte("real lrzsz interop test data. "), 500)
	srcFile := filepath.Join(srcDir, "interop.txt")
	if err := os.WriteFile(srcFile, content, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	conn, cmd := spawnPiped(t, dstDir, rzPath, "--binary", "--quiet")
	defer func() { _ = cmd.Process.Kill() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := New().Send(ctx, conn, []string{srcFile}, nil); err != nil {
		conn.Close()
		t.Fatalf("Send: %v", err)
	}
	conn.Close()
	_ = cmd.Wait()

	got, err := os.ReadFile(filepath.Join(dstDir, "interop.txt"))
	if err != nil {
		t.Fatalf("read file rz received: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
}

// TestLrzszInterop_ReceiveFromRealSz proves our Receive() correctly parses
// what a real sender emits.
func TestLrzszInterop_ReceiveFromRealSz(t *testing.T) {
	szPath := requireBinary(t, "sz")

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	content := bytes.Repeat([]byte("real sz sending to our engine. "), 500)
	srcFile := filepath.Join(srcDir, "fromsz.txt")
	if err := os.WriteFile(srcFile, content, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	conn, cmd := spawnPiped(t, srcDir, szPath, "--binary", "--quiet", "fromsz.txt")
	defer func() { _ = cmd.Process.Kill() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	saved, err := New().Receive(ctx, conn, dstDir, nil)
	conn.Close()
	_ = cmd.Wait()
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved file, got %d: %v", len(saved), saved)
	}

	got, err := os.ReadFile(saved[0])
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
}
