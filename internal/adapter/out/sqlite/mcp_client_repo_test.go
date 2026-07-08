package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func newTestMCPClientRepo(t *testing.T) *MCPClientRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewMCPClientRepo(db)
}

func TestMCPClientRepo_SaveFindByTokenHash(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	client := domain.MCPClient{ClientID: "c1", Name: "claude-desktop", PairedAt: time.Now().Truncate(time.Second).UTC()}

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
	if got.ClientID != client.ClientID || got.Name != client.Name {
		t.Fatalf("FindByTokenHash() = %+v, want %+v", got, client)
	}
	if got.LastSeenAt != nil {
		t.Fatalf("expected nil LastSeenAt, got %v", got.LastSeenAt)
	}
	if got.Revoked {
		t.Fatal("expected a freshly saved client to not be revoked")
	}
}

func TestMCPClientRepo_FindByTokenHash_NotFound(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	_, found, err := repo.FindByTokenHash("nonexistent")
	if err != nil {
		t.Fatalf("FindByTokenHash() error = %v", err)
	}
	if found {
		t.Fatal("expected not found")
	}
}

func TestMCPClientRepo_List(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	if err := repo.Save(domain.MCPClient{ClientID: "c1", Name: "a", PairedAt: older}, "h1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.Save(domain.MCPClient{ClientID: "c2", Name: "b", PairedAt: newer}, "h2"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 || list[0].ClientID != "c2" || list[1].ClientID != "c1" {
		t.Fatalf("List() = %+v, want [c2 c1] (paired_at DESC)", list)
	}
}

func TestMCPClientRepo_TouchSeen(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	if err := repo.Save(domain.MCPClient{ClientID: "c1", Name: "a", PairedAt: time.Now()}, "h1"); err != nil {
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

// TestMCPClientRepo_Revoke_KeepsRowSoftDeleted is the key contract for this
// cycle's design decision: Revoke flags the row rather than removing it,
// so a later FindByTokenHash can still resolve the client (e.g. for audit,
// E3) while reporting Revoked=true for the caller to reject.
func TestMCPClientRepo_Revoke_KeepsRowSoftDeleted(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	if err := repo.Save(domain.MCPClient{ClientID: "c1", Name: "a", PairedAt: time.Now()}, "h1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := repo.Revoke("c1"); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	got, found, err := repo.FindByTokenHash("h1")
	if err != nil {
		t.Fatalf("FindByTokenHash() error = %v", err)
	}
	if !found {
		t.Fatal("expected the revoked row to still be found (soft delete)")
	}
	if !got.Revoked {
		t.Fatal("expected Revoked = true after Revoke()")
	}
}

func TestMCPClientRepo_Revoke_NotFound(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	if err := repo.Revoke("nonexistent"); err != ErrMCPClientNotFound {
		t.Fatalf("Revoke() error = %v, want ErrMCPClientNotFound", err)
	}
}

// TestMCPClientRepo_Delete_RemovesRowEntirely confirms Delete is a hard
// delete, distinct from the soft Revoke above.
func TestMCPClientRepo_Delete_RemovesRowEntirely(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	if err := repo.Save(domain.MCPClient{ClientID: "c1", Name: "a", PairedAt: time.Now()}, "h1"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := repo.Delete("c1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, found, err := repo.FindByTokenHash("h1")
	if err != nil {
		t.Fatalf("FindByTokenHash() error = %v", err)
	}
	if found {
		t.Fatal("expected the deleted row to be gone entirely")
	}
}

func TestMCPClientRepo_Delete_NotFound(t *testing.T) {
	repo := newTestMCPClientRepo(t)
	if err := repo.Delete("nonexistent"); err != ErrMCPClientNotFound {
		t.Fatalf("Delete() error = %v, want ErrMCPClientNotFound", err)
	}
}
