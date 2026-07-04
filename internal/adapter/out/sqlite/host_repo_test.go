package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func newTestDB(t *testing.T) *HostRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewHostRepo(db)
}

func testHost(id string) domain.Host {
	now := time.Now().Truncate(time.Second).UTC()
	return domain.Host{
		ID:        id,
		Name:      "web-prod-01",
		Address:   "10.0.1.15",
		Port:      22,
		Labels:    []string{"prod", "web"},
		Username:  "deploy",
		AuthType:  domain.AuthPassword,
		Source:    "local",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestHostRepo_SaveAndGet_RoundTrips(t *testing.T) {
	repo := newTestDB(t)
	want := testHost("h1")

	if _, err := repo.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.Get("h1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != want.Name || got.Address != want.Address || got.Port != want.Port ||
		got.Username != want.Username || got.AuthType != want.AuthType ||
		!got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("Get() = %+v, want %+v", got, want)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "prod" || got.Labels[1] != "web" {
		t.Fatalf("Get() labels = %v, want [prod web]", got.Labels)
	}
	if got.LastConnectedAt != nil {
		t.Fatalf("expected nil LastConnectedAt, got %v", got.LastConnectedAt)
	}
}

func TestHostRepo_Save_UpsertsOnConflictID(t *testing.T) {
	repo := newTestDB(t)
	h := testHost("h1")
	if _, err := repo.Save(h); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}

	h.Name = "renamed"
	if _, err := repo.Save(h); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}

	all, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 host after upsert, got %d", len(all))
	}
	if all[0].Name != "renamed" {
		t.Fatalf("expected updated name, got %q", all[0].Name)
	}
}

func TestHostRepo_Get_NotFound(t *testing.T) {
	repo := newTestDB(t)
	if _, err := repo.Get("missing"); err != ErrHostNotFound {
		t.Fatalf("expected ErrHostNotFound, got %v", err)
	}
}

func TestHostRepo_Delete(t *testing.T) {
	repo := newTestDB(t)
	_, _ = repo.Save(testHost("h1"))

	if err := repo.Delete("h1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get("h1"); err != ErrHostNotFound {
		t.Fatalf("expected host gone after delete, got err=%v", err)
	}
	if err := repo.Delete("h1"); err != ErrHostNotFound {
		t.Fatalf("expected ErrHostNotFound deleting again, got %v", err)
	}
}

func TestHostRepo_TouchConnected(t *testing.T) {
	repo := newTestDB(t)
	_, _ = repo.Save(testHost("h1"))

	if err := repo.TouchConnected("h1"); err != nil {
		t.Fatalf("TouchConnected() error = %v", err)
	}

	got, err := repo.Get("h1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.LastConnectedAt == nil {
		t.Fatal("expected LastConnectedAt to be set")
	}
}

func TestHostRepo_ListLabels_DedupesAcrossHosts(t *testing.T) {
	repo := newTestDB(t)
	a := testHost("h1")
	a.Labels = []string{"prod", "web"}
	_, _ = repo.Save(a)

	b := testHost("h2")
	b.Labels = []string{"prod", "db"}
	_, _ = repo.Save(b)

	labels, err := repo.ListLabels()
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	seen := map[string]bool{}
	for _, l := range labels {
		seen[l] = true
	}
	if len(seen) != 3 || !seen["prod"] || !seen["web"] || !seen["db"] {
		t.Fatalf("ListLabels() = %v, want distinct [prod web db]", labels)
	}
}

func TestHostRepo_PersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	repo1 := NewHostRepo(db1)
	_, _ = repo1.Save(testHost("h1"))
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer db2.Close()
	repo2 := NewHostRepo(db2)

	got, err := repo2.Get("h1")
	if err != nil {
		t.Fatalf("Get() after reopen error = %v", err)
	}
	if got.ID != "h1" {
		t.Fatalf("expected host to survive reopen, got %+v", got)
	}
}
