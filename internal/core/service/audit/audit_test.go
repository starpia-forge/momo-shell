package audit

import (
	"testing"
	"time"

	"momo-shell/internal/adapter/out/secretscan"
	"momo-shell/internal/core/domain"
)

type fakeRepo struct {
	events  []domain.AuditEvent
	outputs map[int64][]byte
	nextID  int64
}

func (f *fakeRepo) Append(e domain.AuditEvent) (int64, error) {
	f.nextID++
	e.ID = f.nextID
	f.events = append(f.events, e)
	return e.ID, nil
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

func (f *fakeRepo) UpdateOutputRef(auditID int64, ciphertext []byte) error {
	if f.outputs == nil {
		f.outputs = make(map[int64][]byte)
	}
	f.outputs[auditID] = ciphertext
	return nil
}

func (f *fakeRepo) LoadOutput(auditID int64) ([]byte, error) {
	return f.outputs[auditID], nil
}

func (f *fakeRepo) ClearOutputsBefore(cutoffUnix int64) (int64, error) {
	var cleared int64
	for _, e := range f.events {
		if e.ID != 0 && f.outputs[e.ID] != nil && e.Timestamp.Unix() < cutoffUnix {
			delete(f.outputs, e.ID)
			cleared++
		}
	}
	return cleared, nil
}

func TestService_RecordQuery(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, newFakeSecretStore(), secretscan.New())

	if _, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindConnect, Decision: "granted"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if _, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand, Decision: "auto"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if _, err := svc.Record(domain.AuditEvent{SessionID: "s2", Kind: domain.AuditKindConnect, Decision: "granted"}); err != nil {
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

// TestService_AttachLoadRoundtrip confirms AttachOutput encrypts before
// handing the blob to the repo (E3-b's core invariant -- the DB never holds
// plaintext output) and that LoadOutput decrypts it back correctly.
func TestService_AttachLoadRoundtrip(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, newFakeSecretStore(), secretscan.New())

	id, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand, Decision: "auto"})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	plaintext := []byte("$ echo hi\nhi\n")
	if err := svc.AttachOutput(id, plaintext); err != nil {
		t.Fatalf("AttachOutput() error = %v", err)
	}

	stored := repo.outputs[id]
	if stored == nil {
		t.Fatal("expected the repo to receive a stored blob")
	}
	if string(stored) == string(plaintext) {
		t.Fatal("repo received plaintext, want ciphertext -- AttachOutput must encrypt before storing")
	}

	got, err := svc.LoadOutput(id)
	if err != nil {
		t.Fatalf("LoadOutput() error = %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("LoadOutput() = %q, want %q", got, plaintext)
	}
}

// TestService_LoadOutput_NoneAttachedReturnsNil confirms an audit row with
// no captured output (a connect/control event, or a command whose capture
// hasn't sealed yet) reports (nil, nil) rather than an error.
func TestService_LoadOutput_NoneAttachedReturnsNil(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, newFakeSecretStore(), secretscan.New())

	id, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	got, err := svc.LoadOutput(id)
	if err != nil || got != nil {
		t.Fatalf("LoadOutput() = (%v, %v), want (nil, nil)", got, err)
	}
}

// TestService_PurgeOutputsBefore confirms retention expiry nulls the output
// capture past outputRetention while the decision row survives.
func TestService_PurgeOutputsBefore(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, newFakeSecretStore(), secretscan.New())

	oldID, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand, Timestamp: time.Unix(1000, 0)})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := svc.AttachOutput(oldID, []byte("old output")); err != nil {
		t.Fatalf("AttachOutput() error = %v", err)
	}

	if err := svc.PurgeOutputsBefore(time.Unix(1000, 0).Add(outputRetention + time.Hour)); err != nil {
		t.Fatalf("PurgeOutputsBefore() error = %v", err)
	}

	if repo.outputs[oldID] != nil {
		t.Fatal("expected output to be purged once past retention")
	}
	if len(repo.events) != 1 {
		t.Fatalf("expected the decision row to survive purge, got %d events", len(repo.events))
	}
}

// TestService_LoadOutputSegments_RedactsAndPreservesValue confirms E5b's
// splice runs the byte-offset walk on the decrypted []byte before any string
// conversion -- the surrounding text is valid multibyte UTF-8 (Korean), so a
// rune-index (rather than byte-offset) splice would corrupt it.
func TestService_LoadOutputSegments_RedactsAndPreservesValue(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, newFakeSecretStore(), secretscan.New())

	id, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	const secret = "AKIAABCDEFGHIJKLMNOP"
	plaintext := []byte("안녕 " + secret + " 감사")
	if err := svc.AttachOutput(id, plaintext); err != nil {
		t.Fatalf("AttachOutput() error = %v", err)
	}

	segments, err := svc.LoadOutputSegments(id)
	if err != nil {
		t.Fatalf("LoadOutputSegments() error = %v", err)
	}
	if len(segments) != 3 {
		t.Fatalf("LoadOutputSegments() returned %d segments, want 3: %+v", len(segments), segments)
	}
	if segments[0].Redacted || segments[0].Text != "안녕 " {
		t.Fatalf("segments[0] = %+v, want plain %q", segments[0], "안녕 ")
	}
	if !segments[1].Redacted || segments[1].Type != "aws-access-key-id" || segments[1].Value != secret {
		t.Fatalf("segments[1] = %+v, want redacted aws-access-key-id with value %q", segments[1], secret)
	}
	if segments[2].Redacted || segments[2].Text != " 감사" {
		t.Fatalf("segments[2] = %+v, want plain %q", segments[2], " 감사")
	}
}

// TestService_LoadOutputSegments_NoHitsReturnsSinglePlainSegment confirms
// output with nothing to redact still round-trips as one Text-only segment
// rather than an empty slice.
func TestService_LoadOutputSegments_NoHitsReturnsSinglePlainSegment(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, newFakeSecretStore(), secretscan.New())

	id, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := svc.AttachOutput(id, []byte("$ echo hi\nhi\n")); err != nil {
		t.Fatalf("AttachOutput() error = %v", err)
	}

	segments, err := svc.LoadOutputSegments(id)
	if err != nil {
		t.Fatalf("LoadOutputSegments() error = %v", err)
	}
	if len(segments) != 1 || segments[0].Redacted || segments[0].Text != "$ echo hi\nhi\n" {
		t.Fatalf("LoadOutputSegments() = %+v, want a single plain segment", segments)
	}
}

// TestService_LoadOutputSegments_NoCaptureReturnsNil mirrors
// TestService_LoadOutput_NoneAttachedReturnsNil -- no capture means no
// segments, not an error.
func TestService_LoadOutputSegments_NoCaptureReturnsNil(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, newFakeSecretStore(), secretscan.New())

	id, err := svc.Record(domain.AuditEvent{SessionID: "s1", Kind: domain.AuditKindCommand})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	segments, err := svc.LoadOutputSegments(id)
	if err != nil || segments != nil {
		t.Fatalf("LoadOutputSegments() = (%v, %v), want (nil, nil)", segments, err)
	}
}
