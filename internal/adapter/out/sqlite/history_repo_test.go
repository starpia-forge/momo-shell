package sqlite

import (
	"path/filepath"
	"testing"

	"momo-shell/internal/core/domain"
)

func newTestHistoryRepo(t *testing.T) *HistoryRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewHistoryRepo(db)
}

func strPtr(s string) *string { return &s }

func TestHistoryRepo_Append_AssignsID(t *testing.T) {
	repo := newTestHistoryRepo(t)

	entry, err := repo.Append(nil, "ls -la", 1000)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("expected a non-zero assigned ID")
	}
	if entry.Command != "ls -la" || entry.ExecutedAt != 1000 || entry.HostID != nil {
		t.Fatalf("Append() = %+v, unexpected fields", entry)
	}
}

func TestHistoryRepo_List_OrdersByExecutedAtDesc(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "first", 1000)
	_, _ = repo.Append(nil, "second", 2000)
	_, _ = repo.Append(nil, "third", 3000)

	got, err := repo.List(domain.HistoryQuery{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 3 || got[0].Command != "third" || got[1].Command != "second" || got[2].Command != "first" {
		t.Fatalf("List() = %+v, want most-recent-first order", got)
	}
}

func TestHistoryRepo_List_FiltersByHostID(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "local cmd", 1000)
	_, _ = repo.Append(strPtr("host-1"), "remote cmd", 2000)

	got, err := repo.List(domain.HistoryQuery{HostID: strPtr("host-1")})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Command != "remote cmd" {
		t.Fatalf("List() = %+v, want only host-1 entries", got)
	}
}

func TestHistoryRepo_List_FiltersToLocalOnly(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "local cmd", 1000)
	_, _ = repo.Append(strPtr("host-1"), "remote cmd", 2000)

	got, err := repo.List(domain.HistoryQuery{HostID: strPtr("")})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Command != "local cmd" {
		t.Fatalf("List() = %+v, want only the local entry (HostID: ptr(\"\") means local-only)", got)
	}
}

func TestHistoryRepo_List_FiltersBySearch(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "docker compose up", 1000)
	_, _ = repo.Append(nil, "systemctl restart nginx", 2000)

	got, err := repo.List(domain.HistoryQuery{Search: "nginx"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Command != "systemctl restart nginx" {
		t.Fatalf("List() = %+v, want only the nginx entry", got)
	}
}

func TestHistoryRepo_List_RespectsLimitOffset(t *testing.T) {
	repo := newTestHistoryRepo(t)
	for i, ts := range []int64{1000, 2000, 3000} {
		_, _ = repo.Append(nil, string(rune('a'+i)), ts)
	}

	got, err := repo.List(domain.HistoryQuery{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Command != "b" { // b was executed at 2000, second-most-recent
		t.Fatalf("List() = %+v, want single second-most-recent entry", got)
	}
}

func TestHistoryRepo_TouchLast_UpdatesTimestamp(t *testing.T) {
	repo := newTestHistoryRepo(t)
	entry, _ := repo.Append(nil, "ls", 1000)

	if err := repo.TouchLast(entry.ID, 5000); err != nil {
		t.Fatalf("TouchLast() error = %v", err)
	}

	got, err := repo.List(domain.HistoryQuery{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].ExecutedAt != 5000 {
		t.Fatalf("List() = %+v, want ExecutedAt updated to 5000 with no new row", got)
	}
}

func TestHistoryRepo_TouchLast_NotFound(t *testing.T) {
	repo := newTestHistoryRepo(t)
	if err := repo.TouchLast(999, 1000); err != ErrHistoryEntryNotFound {
		t.Fatalf("expected ErrHistoryEntryNotFound, got %v", err)
	}
}

func TestHistoryRepo_LastForHost_ReturnsMostRecent(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "old", 1000)
	_, _ = repo.Append(nil, "new", 2000)

	got, found, err := repo.LastForHost(nil)
	if err != nil {
		t.Fatalf("LastForHost() error = %v", err)
	}
	if !found || got.Command != "new" {
		t.Fatalf("LastForHost() = %+v, found=%v, want the most recent local entry", got, found)
	}
}

func TestHistoryRepo_LastForHost_NoneFound(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, found, err := repo.LastForHost(strPtr("no-such-host"))
	if err != nil {
		t.Fatalf("LastForHost() error = %v", err)
	}
	if found {
		t.Fatal("expected found=false when no entries exist for the host")
	}
}

func TestHistoryRepo_Delete(t *testing.T) {
	repo := newTestHistoryRepo(t)
	entry, _ := repo.Append(nil, "ls", 1000)

	if err := repo.Delete(entry.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	got, err := repo.List(domain.HistoryQuery{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no entries after delete, got %+v", got)
	}
	if err := repo.Delete(entry.ID); err != ErrHistoryEntryNotFound {
		t.Fatalf("expected ErrHistoryEntryNotFound deleting again, got %v", err)
	}
}

func TestHistoryRepo_Clear_ByHost(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "local cmd", 1000)
	_, _ = repo.Append(strPtr("host-1"), "remote cmd", 2000)

	if err := repo.Clear(strPtr("host-1")); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	got, err := repo.List(domain.HistoryQuery{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Command != "local cmd" {
		t.Fatalf("List() = %+v, want only the local entry left", got)
	}
}

func TestHistoryRepo_Clear_LocalOnly(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "local cmd", 1000)
	_, _ = repo.Append(strPtr("host-1"), "remote cmd", 2000)

	if err := repo.Clear(strPtr("")); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	got, err := repo.List(domain.HistoryQuery{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Command != "remote cmd" {
		t.Fatalf("List() = %+v, want only the remote entry left", got)
	}
}

func TestHistoryRepo_Clear_All(t *testing.T) {
	repo := newTestHistoryRepo(t)
	_, _ = repo.Append(nil, "local cmd", 1000)
	_, _ = repo.Append(strPtr("host-1"), "remote cmd", 2000)

	if err := repo.Clear(nil); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	got, err := repo.List(domain.HistoryQuery{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no entries after clearing all, got %+v", got)
	}
}

func TestHistoryRepo_PersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	repo1 := NewHistoryRepo(db1)
	_, _ = repo1.Append(nil, "ls", 1000)
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer db2.Close()
	repo2 := NewHistoryRepo(db2)

	got, err := repo2.List(domain.HistoryQuery{})
	if err != nil {
		t.Fatalf("List() after reopen error = %v", err)
	}
	if len(got) != 1 || got[0].Command != "ls" {
		t.Fatalf("expected entry to survive reopen, got %+v", got)
	}
}
