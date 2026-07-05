package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func newTestShareClientRepo(t *testing.T) *ShareClientRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewShareClientRepo(db)
}

func TestShareClientRepo_SaveFindByTokenHash(t *testing.T) {
	repo := newTestShareClientRepo(t)
	client := domain.ShareClient{ID: "c1", Name: "kim-laptop", PairedAt: time.Now().Truncate(time.Second).UTC()}

	if err := repo.Save(client, "hash-abc"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, found, err := repo.FindByTokenHash("hash-abc")
	if err != nil {
		t.Fatalf("FindByTokenHash() error = %v", err)
	}
	if !found {
		t.Fatal("expected client to be found")
	}
	if got.ID != client.ID || got.Name != client.Name {
		t.Fatalf("FindByTokenHash() = %+v, want %+v", got, client)
	}
	if got.LastSeenAt != nil {
		t.Fatalf("expected nil LastSeenAt, got %v", got.LastSeenAt)
	}
}

func TestShareClientRepo_FindByTokenHash_NotFound(t *testing.T) {
	repo := newTestShareClientRepo(t)
	_, found, err := repo.FindByTokenHash("nonexistent")
	if err != nil {
		t.Fatalf("FindByTokenHash() error = %v", err)
	}
	if found {
		t.Fatal("expected not found")
	}
}

func TestShareClientRepo_ListAndDelete(t *testing.T) {
	repo := newTestShareClientRepo(t)
	if err := repo.Save(domain.ShareClient{ID: "c1", Name: "a", PairedAt: time.Now()}, "h1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.Save(domain.ShareClient{ID: "c2", Name: "b", PairedAt: time.Now()}, "h2"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() = %+v, want 2 entries", list)
	}

	if err := repo.Delete("c1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := repo.Delete("c1"); err != ErrShareClientNotFound {
		t.Fatalf("Delete() (again) error = %v, want ErrShareClientNotFound", err)
	}

	list, err = repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != "c2" {
		t.Fatalf("List() after delete = %+v, want [c2]", list)
	}
}

func TestShareClientRepo_TouchSeen(t *testing.T) {
	repo := newTestShareClientRepo(t)
	if err := repo.Save(domain.ShareClient{ID: "c1", Name: "a", PairedAt: time.Now()}, "h1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.TouchSeen("c1"); err != nil {
		t.Fatalf("TouchSeen() error = %v", err)
	}

	got, found, err := repo.FindByTokenHash("h1")
	if err != nil || !found {
		t.Fatalf("FindByTokenHash() = %+v, %v, %v", got, found, err)
	}
	if got.LastSeenAt == nil {
		t.Fatal("expected LastSeenAt to be set after TouchSeen")
	}
}

func newTestShareSettingsRepo(t *testing.T) *ShareSettingsRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewShareSettingsRepo(db)
}

func TestShareSettingsRepo_InstanceIDGeneratedOnceAndPersisted(t *testing.T) {
	repo := newTestShareSettingsRepo(t)

	first, err := repo.InstanceID()
	if err != nil {
		t.Fatalf("InstanceID() error = %v", err)
	}
	if first == "" {
		t.Fatal("expected non-empty instance ID")
	}

	second, err := repo.InstanceID()
	if err != nil {
		t.Fatalf("InstanceID() (second call) error = %v", err)
	}
	if first != second {
		t.Fatalf("InstanceID() not stable: %q != %q", first, second)
	}
}

func TestShareSettingsRepo_SharedHostIDsRoundTrips(t *testing.T) {
	repo := newTestShareSettingsRepo(t)

	empty, err := repo.SharedHostIDs()
	if err != nil {
		t.Fatalf("SharedHostIDs() error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("SharedHostIDs() (before set) = %v, want empty", empty)
	}

	if err := repo.SetSharedHostIDs([]string{"h1", "h2"}); err != nil {
		t.Fatalf("SetSharedHostIDs() error = %v", err)
	}
	got, err := repo.SharedHostIDs()
	if err != nil {
		t.Fatalf("SharedHostIDs() error = %v", err)
	}
	if len(got) != 2 || got[0] != "h1" || got[1] != "h2" {
		t.Fatalf("SharedHostIDs() = %v, want [h1 h2]", got)
	}

	if err := repo.SetSharedHostIDs([]string{"h3"}); err != nil {
		t.Fatalf("SetSharedHostIDs() (overwrite) error = %v", err)
	}
	got, err = repo.SharedHostIDs()
	if err != nil {
		t.Fatalf("SharedHostIDs() error = %v", err)
	}
	if len(got) != 1 || got[0] != "h3" {
		t.Fatalf("SharedHostIDs() after overwrite = %v, want [h3]", got)
	}
}
