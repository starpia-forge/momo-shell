package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

func newTestAuditRepo(t *testing.T) *AuditRepo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAuditRepo(db)
}

func TestAuditRepo_AppendList(t *testing.T) {
	repo := newTestAuditRepo(t)
	now := time.Now().Truncate(time.Second).UTC()

	event := domain.AuditEvent{
		Timestamp:   now,
		ClientID:    "client-1",
		SessionID:   "session-1",
		Kind:        domain.AuditKindCommand,
		OriginalCmd: "rm -rf /tmp/x",
		GuardedCmd:  "rm -rf -- /tmp/x",
		Resolve: domain.ResolveSummary{
			Risk:      "medium",
			Uncertain: false,
			Reasons:   []string{"rm: destructive command"},
		},
		Approver: "custodian",
		Decision: "approved",
	}
	if err := repo.Append(event); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got, err := repo.List("session-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() returned %d events, want 1", len(got))
	}
	g := got[0]
	if g.ID == 0 {
		t.Fatal("expected non-zero ID after Append")
	}
	if !g.Timestamp.Equal(now) {
		t.Fatalf("Timestamp = %v, want %v", g.Timestamp, now)
	}
	if g.ClientID != event.ClientID || g.SessionID != event.SessionID || g.Kind != event.Kind {
		t.Fatalf("List() = %+v, want matching %+v", g, event)
	}
	if g.OriginalCmd != event.OriginalCmd || g.GuardedCmd != event.GuardedCmd {
		t.Fatalf("List() command fields = %+v, want %+v", g, event)
	}
	if g.Resolve.Risk != event.Resolve.Risk || g.Resolve.Uncertain != event.Resolve.Uncertain || len(g.Resolve.Reasons) != 1 || g.Resolve.Reasons[0] != event.Resolve.Reasons[0] {
		t.Fatalf("List() resolve summary = %+v, want %+v", g.Resolve, event.Resolve)
	}
	if g.Approver != event.Approver || g.Decision != event.Decision {
		t.Fatalf("List() approver/decision = %+v, want %+v", g, event)
	}
}

func TestAuditRepo_List_SessionFilter(t *testing.T) {
	repo := newTestAuditRepo(t)
	now := time.Now()

	for _, sessionID := range []string{"a", "b", "a"} {
		if err := repo.Append(domain.AuditEvent{Timestamp: now, SessionID: sessionID, Kind: domain.AuditKindCommand}); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	got, err := repo.List("a")
	if err != nil {
		t.Fatalf("List(\"a\") error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List(\"a\") returned %d events, want 2", len(got))
	}

	all, err := repo.List("")
	if err != nil {
		t.Fatalf("List(\"\") error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("List(\"\") returned %d events, want 3", len(all))
	}
}

// TestAuditRepo_List_OrderedByID confirms replay order (US-5) is
// insertion-order (id ASC), not ts -- second-granularity timestamps can tie
// (e.g. a connect immediately followed by a command), which would make
// "ORDER BY ts" non-deterministic.
func TestAuditRepo_List_OrderedByID(t *testing.T) {
	repo := newTestAuditRepo(t)
	sameTS := time.Now().Truncate(time.Second)

	kinds := []domain.AuditKind{domain.AuditKindConnect, domain.AuditKindCommand, domain.AuditKindControl}
	for _, k := range kinds {
		if err := repo.Append(domain.AuditEvent{Timestamp: sameTS, SessionID: "s", Kind: k}); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	got, err := repo.List("s")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != len(kinds) {
		t.Fatalf("List() returned %d events, want %d", len(got), len(kinds))
	}
	for i, k := range kinds {
		if got[i].Kind != k {
			t.Fatalf("List()[%d].Kind = %s, want %s (order must be insertion order despite tied ts)", i, got[i].Kind, k)
		}
		if i > 0 && got[i].ID <= got[i-1].ID {
			t.Fatalf("List()[%d].ID = %d not strictly greater than [%d].ID = %d", i, got[i].ID, i-1, got[i-1].ID)
		}
	}
}
