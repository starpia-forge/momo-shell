package audit

import (
	"testing"

	"momo-shell/internal/core/domain"
)

type fakeRepo struct {
	events []domain.AuditEvent
	nextID int64
}

func (f *fakeRepo) Append(e domain.AuditEvent) error {
	f.nextID++
	e.ID = f.nextID
	f.events = append(f.events, e)
	return nil
}

func (f *fakeRepo) List(sessionID string) ([]domain.AuditEvent, error) {
	if sessionID == "" {
		return f.events, nil
	}
	var out []domain.AuditEvent
	for _, e := range f.events {
		if e.SessionID == sessionID {
			out = append(out, e)
		}
	}
	return out, nil
}

func TestService_RecordQuery(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo)

	if err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindConnect, Decision: "granted"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand, Decision: "auto"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := svc.Record(domain.AuditEvent{SessionID: "s2", Kind: domain.AuditKindConnect, Decision: "granted"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	got, err := svc.Query("s1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Query(\"s1\") returned %d events, want 2", len(got))
	}
	// id-ascending order is the replay sequence (US-5).
	if got[0].Kind != domain.AuditKindConnect || got[1].Kind != domain.AuditKindCommand {
		t.Fatalf("Query(\"s1\") order = [%s, %s], want [connect, command]", got[0].Kind, got[1].Kind)
	}
	if got[0].ID >= got[1].ID {
		t.Fatalf("Query(\"s1\")[0].ID = %d not less than [1].ID = %d", got[0].ID, got[1].ID)
	}
}
