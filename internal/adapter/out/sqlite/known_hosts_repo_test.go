package sqlite

import (
	"path/filepath"
	"testing"
)

func newTestKnownHostsRepo(t *testing.T) *KnownHostsRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewKnownHostsRepo(db)
}

func TestKnownHostsRepo_GetMissing(t *testing.T) {
	repo := newTestKnownHostsRepo(t)

	_, found, err := repo.Get("10.0.1.15", 22, "ssh-ed25519")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if found {
		t.Fatal("expected not found for unregistered host")
	}
}

func TestKnownHostsRepo_PutAndGet_RoundTrips(t *testing.T) {
	repo := newTestKnownHostsRepo(t)

	if err := repo.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:abc123"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	fingerprint, found, err := repo.Get("10.0.1.15", 22, "ssh-ed25519")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || fingerprint != "SHA256:abc123" {
		t.Fatalf("Get() = (%q, %v), want (SHA256:abc123, true)", fingerprint, found)
	}
}

func TestKnownHostsRepo_Put_UpsertsOnConflict(t *testing.T) {
	repo := newTestKnownHostsRepo(t)
	_ = repo.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:old")

	if err := repo.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:new"); err != nil {
		t.Fatalf("Put() overwrite error = %v", err)
	}

	fingerprint, found, err := repo.Get("10.0.1.15", 22, "ssh-ed25519")
	if err != nil || !found {
		t.Fatalf("Get() after overwrite: found=%v err=%v", found, err)
	}
	if fingerprint != "SHA256:new" {
		t.Fatalf("expected overwritten fingerprint, got %q", fingerprint)
	}
}

func TestKnownHostsRepo_Delete(t *testing.T) {
	repo := newTestKnownHostsRepo(t)
	_ = repo.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:abc123")

	if err := repo.Delete("10.0.1.15", 22, "ssh-ed25519"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, found, err := repo.Get("10.0.1.15", 22, "ssh-ed25519")
	if err != nil {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if found {
		t.Fatal("expected not found after delete")
	}
}

func TestKnownHostsRepo_DifferentAlgoIsDistinctEntry(t *testing.T) {
	repo := newTestKnownHostsRepo(t)
	_ = repo.Put("10.0.1.15", 22, "ssh-ed25519", "SHA256:ed25519fp")
	_ = repo.Put("10.0.1.15", 22, "rsa-sha2-512", "SHA256:rsafp")

	fp, found, err := repo.Get("10.0.1.15", 22, "rsa-sha2-512")
	if err != nil || !found || fp != "SHA256:rsafp" {
		t.Fatalf("Get(rsa) = (%q, %v, %v)", fp, found, err)
	}
	fp, found, err = repo.Get("10.0.1.15", 22, "ssh-ed25519")
	if err != nil || !found || fp != "SHA256:ed25519fp" {
		t.Fatalf("Get(ed25519) = (%q, %v, %v)", fp, found, err)
	}
}
