//go:build !windows

package mcpipc

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestSocketPath_UsesXDGRuntimeDirWhenSet(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	got, err := socketPath()
	if err != nil {
		t.Fatalf("socketPath() error = %v", err)
	}
	want := filepath.Join("/run/user/1000", "momo-shell", "mcp.sock")
	if got != want {
		t.Errorf("socketPath() = %q, want %q", got, want)
	}
}

func TestSocketPath_FallsBackToHomeWhenXDGUnset(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir() unavailable: %v", err)
	}
	got, err := socketPath()
	if err != nil {
		t.Fatalf("socketPath() error = %v", err)
	}
	want := filepath.Join(home, ".momo-shell", "mcp.sock")
	if got != want {
		t.Errorf("socketPath() = %q, want %q", got, want)
	}
}

func TestListen_CreatesSocketWithRestrictivePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	l, err := listen()
	if err != nil {
		t.Fatalf("listen() error = %v", err)
	}
	defer l.Close()

	path, err := socketPath()
	if err != nil {
		t.Fatalf("socketPath() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(socket) error = %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("socket mode = %o, want 0600", mode)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0700 {
		t.Errorf("dir mode = %o, want 0700", mode)
	}
}

func TestListen_RemovesStaleSocketAndRebinds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	l1, err := listen()
	if err != nil {
		t.Fatalf("first listen() error = %v", err)
	}
	l1.Close()

	// Simulate a crashed process leaving the socket file behind (a normal
	// Close on a unix listener removes its own path, so recreate one to
	// exercise the stale-file-removal branch in listen()).
	path, err := socketPath()
	if err != nil {
		t.Fatalf("socketPath() error = %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(stale socket file) error = %v", err)
	}
	f.Close()

	l2, err := listen()
	if err != nil {
		t.Fatalf("second listen() with a stale socket present, error = %v", err)
	}
	defer l2.Close()
}

func TestListen_ClientCanConnectAndRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	l, err := listen()
	if err != nil {
		t.Fatalf("listen() error = %v", err)
	}
	defer l.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		buf := make([]byte, 5)
		if _, err := conn.Read(buf); err != nil {
			serverErr <- err
			return
		}
		if _, err := conn.Write(buf); err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	path, err := socketPath()
	if err != nil {
		t.Fatalf("socketPath() error = %v", err)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	buf := make([]byte, 5)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(buf) != "hello" {
		t.Errorf("roundtrip = %q, want %q", buf, "hello")
	}
	if err := <-serverErr; err != nil {
		t.Errorf("server goroutine error = %v", err)
	}
}
