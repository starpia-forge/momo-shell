package sshconn

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"momo-shell/internal/adapter/out/sftp"
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/out"
)

// TestOpenFileSystem_RoundTrip exercises the real SFTP wire protocol (no
// docker required): a genuine pkg/sftp.Server answers a "sftp" subsystem
// request over the same in-process SSH connection opener.Open already uses
// for the shell, proving OpenFileSystem reaches it via the injected
// WithFileSystemFactory rather than a second connection.
func TestOpenFileSystem_RoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	srv.sftpRoot = root
	host := srv.host(domain.AuthPassword, "")

	opener := New(WithFileSystemFactory(sftp.NewFromClient))
	stream, err := opener.Open(host, "correct-password", trustAllVerifier, 80, 24)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	capable, ok := stream.(out.FileSystemCapable)
	if !ok {
		t.Fatalf("sshStream does not implement out.FileSystemCapable")
	}
	fs, err := capable.OpenFileSystem()
	if err != nil {
		t.Fatalf("OpenFileSystem() error = %v", err)
	}
	defer fs.Close()

	entries, err := fs.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "hello.txt" {
		t.Fatalf("unexpected entries: %+v", entries)
	}

	rc, err := fs.Open("hello.txt")
	if err != nil {
		t.Fatalf("Open(hello.txt) error = %v", err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		t.Fatalf("read hello.txt: %v", err)
	}
	if buf.String() != "hi" {
		t.Fatalf("content mismatch: got %q", buf.String())
	}
}

// TestOpenFileSystem_UnavailableWithoutFactory covers the fallback signal
// TransferService relies on to offer the rz path: an Opener built without
// WithFileSystemFactory (e.g. a caller that only needs terminal access)
// must fail fast, without even attempting a subsystem channel.
func TestOpenFileSystem_UnavailableWithoutFactory(t *testing.T) {
	srv := newTestSSHServer(t, passwordServerConfig("correct-password"))
	host := srv.host(domain.AuthPassword, "")

	stream, err := New().Open(host, "correct-password", trustAllVerifier, 80, 24)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close()

	capable, ok := stream.(out.FileSystemCapable)
	if !ok {
		t.Fatalf("sshStream does not implement out.FileSystemCapable")
	}
	if _, err := capable.OpenFileSystem(); err == nil {
		t.Fatalf("expected error when Opener has no file system factory configured")
	}
}
