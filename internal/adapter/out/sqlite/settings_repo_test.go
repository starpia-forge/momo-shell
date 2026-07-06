package sqlite

import (
	"path/filepath"
	"testing"
)

func newTestSettingsRepo(t *testing.T) *SettingsRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSettingsRepo(db)
}

func TestSettingsRepo_GetNotFound(t *testing.T) {
	repo := newTestSettingsRepo(t)
	_, found, err := repo.Get("appearance.theme")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if found {
		t.Fatal("expected not found")
	}
}

func TestSettingsRepo_SetGetRoundTrips(t *testing.T) {
	repo := newTestSettingsRepo(t)
	if err := repo.Set("appearance.theme", "light"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, found, err := repo.Get("appearance.theme")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || value != "light" {
		t.Fatalf("Get() = (%q, %v), want (light, true)", value, found)
	}

	if err := repo.Set("appearance.theme", "dark"); err != nil {
		t.Fatalf("Set() (overwrite) error = %v", err)
	}
	value, found, err = repo.Get("appearance.theme")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || value != "dark" {
		t.Fatalf("Get() after overwrite = (%q, %v), want (dark, true)", value, found)
	}
}

func TestSettingsRepo_All(t *testing.T) {
	repo := newTestSettingsRepo(t)
	if err := repo.Set("appearance.theme", "light"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := repo.Set("terminal.fontSize", "16"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	all, err := repo.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(all) != 2 || all["appearance.theme"] != "light" || all["terminal.fontSize"] != "16" {
		t.Fatalf("All() = %+v, want map[appearance.theme:light terminal.fontSize:16]", all)
	}
}
