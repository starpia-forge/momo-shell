package keychain

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newTestFileStore(t *testing.T) *fileStore {
	t.Helper()
	dir := t.TempDir()
	return &fileStore{
		dataPath: filepath.Join(dir, "secrets.enc"),
		keyPath:  filepath.Join(dir, "secrets.key"),
	}
}

func TestFileStore_SetAndGet_RoundTrips(t *testing.T) {
	fs := newTestFileStore(t)

	if err := fs.Set("host:1", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := fs.Get("host:1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "hunter2" {
		t.Fatalf("Get() = %q, want %q", got, "hunter2")
	}
}

func TestFileStore_Get_NotFound(t *testing.T) {
	fs := newTestFileStore(t)

	if _, err := fs.Get("missing"); err != ErrSecretNotFound {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestFileStore_Delete(t *testing.T) {
	fs := newTestFileStore(t)
	_ = fs.Set("host:1", "hunter2")

	if err := fs.Delete("host:1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := fs.Get("host:1"); err != ErrSecretNotFound {
		t.Fatalf("expected ErrSecretNotFound after delete, got %v", err)
	}
}

func TestFileStore_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "secrets.enc")
	keyPath := filepath.Join(dir, "secrets.key")

	fs1 := &fileStore{dataPath: dataPath, keyPath: keyPath}
	if err := fs1.Set("host:1", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	fs2 := &fileStore{dataPath: dataPath, keyPath: keyPath}
	got, err := fs2.Get("host:1")
	if err != nil {
		t.Fatalf("Get() from a fresh instance error = %v", err)
	}
	if got != "hunter2" {
		t.Fatalf("Get() = %q, want %q", got, "hunter2")
	}
}

func TestFileStore_TamperedFile_FailsToDecrypt(t *testing.T) {
	fs := newTestFileStore(t)
	if err := fs.Set("host:1", "hunter2"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	data, err := os.ReadFile(fs.dataPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	data[len(data)-1] ^= 0xFF // flip a byte in the GCM tag/ciphertext
	if err := os.WriteFile(fs.dataPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := fs.Get("host:1"); err == nil {
		t.Fatal("expected decrypt error on tampered secrets file, got nil")
	}
}

func TestFileStore_KeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits aren't meaningful on Windows")
	}
	fs := newTestFileStore(t)
	_ = fs.Set("host:1", "hunter2")

	info, err := os.Stat(fs.keyPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected key file mode 0600, got %o", perm)
	}
}
