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
	id, err := repo.Append(event)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if id == 0 {
		t.Fatal("Append() returned id 0, want non-zero")
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
		if _, err := repo.Append(domain.AuditEvent{Timestamp: now, SessionID: sessionID, Kind: domain.AuditKindCommand}); err != nil {
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
		if _, err := repo.Append(domain.AuditEvent{Timestamp: sameTS, SessionID: "s", Kind: k}); err != nil {
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

// TestAuditRepo_Append_ReturnsID confirms Append's returned id identifies
// the exact row just inserted -- E3-b's capture tap links output to this id
// via UpdateOutputRef, so it must be race-free against concurrent Appends
// (LastInsertId), not a MAX(id) lookup that could resolve to the wrong row.
func TestAuditRepo_Append_ReturnsID(t *testing.T) {
	repo := newTestAuditRepo(t)

	id1, err := repo.Append(domain.AuditEvent{Timestamp: time.Now(), SessionID: "s", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	id2, err := repo.Append(domain.AuditEvent{Timestamp: time.Now(), SessionID: "s", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if id1 == 0 || id2 == 0 || id2 <= id1 {
		t.Fatalf("Append() ids = (%d, %d), want two distinct increasing non-zero ids", id1, id2)
	}

	got, err := repo.List("s")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != id1 || got[1].ID != id2 {
		t.Fatalf("List() ids = [%d, %d], want [%d, %d]", got[0].ID, got[1].ID, id1, id2)
	}
}

// TestAuditRepo_OutputRefRoundtrip confirms UpdateOutputRef/LoadOutput
// round-trip a blob (E3-b), that a row with none attached reports (nil,
// nil), and that List never surfaces output_ref -- the AI-facing replay
// path (Query -> List -> scanAuditEvent) has no route to it.
func TestAuditRepo_OutputRefRoundtrip(t *testing.T) {
	repo := newTestAuditRepo(t)

	id, err := repo.Append(domain.AuditEvent{Timestamp: time.Now(), SessionID: "s", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if out, err := repo.LoadOutput(id); err != nil || out != nil {
		t.Fatalf("LoadOutput() before attach = (%v, %v), want (nil, nil)", out, err)
	}

	want := []byte("simulated ciphertext bytes")
	if err := repo.UpdateOutputRef(id, want); err != nil {
		t.Fatalf("UpdateOutputRef() error = %v", err)
	}

	got, err := repo.LoadOutput(id)
	if err != nil {
		t.Fatalf("LoadOutput() error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("LoadOutput() = %q, want %q", got, want)
	}

	events, err := repo.List("s")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(events) != 1 || events[0].ID != id {
		t.Fatalf("List() = %+v, want the one event with id %d", events, id)
	}
}

// TestAuditRepo_ClearOutputsBefore confirms retention expiry nulls only
// output_ref for rows older than the cutoff that still have one attached --
// the decision row itself always survives -- and reports the count cleared.
func TestAuditRepo_ClearOutputsBefore(t *testing.T) {
	repo := newTestAuditRepo(t)

	oldID, err := repo.Append(domain.AuditEvent{Timestamp: time.Unix(1000, 0), SessionID: "s", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	newID, err := repo.Append(domain.AuditEvent{Timestamp: time.Unix(5000, 0), SessionID: "s", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	for _, id := range []int64{oldID, newID} {
		if err := repo.UpdateOutputRef(id, []byte("ciphertext")); err != nil {
			t.Fatalf("UpdateOutputRef(%d) error = %v", id, err)
		}
	}

	cleared, err := repo.ClearOutputsBefore(2000)
	if err != nil {
		t.Fatalf("ClearOutputsBefore() error = %v", err)
	}
	if cleared != 1 {
		t.Fatalf("ClearOutputsBefore() cleared = %d, want 1", cleared)
	}

	if out, err := repo.LoadOutput(oldID); err != nil || out != nil {
		t.Fatalf("LoadOutput(oldID) after purge = (%v, %v), want (nil, nil)", out, err)
	}
	if out, err := repo.LoadOutput(newID); err != nil || string(out) != "ciphertext" {
		t.Fatalf("LoadOutput(newID) after purge = (%v, %v), want (\"ciphertext\", nil)", out, err)
	}

	events, err := repo.List("s")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("List() returned %d events, want 2 (decision rows survive output purge)", len(events))
	}
}
