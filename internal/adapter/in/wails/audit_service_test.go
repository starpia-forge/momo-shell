package wails

import (
	"testing"
	"time"

	"momo-shell/internal/core/domain"
)

// fakeAuditUseCase is a hand-rolled in.AuditUseCase (house style: no mock
// framework, mirrors audit/mocks_test.go's fakeSecretStore).
type fakeAuditUseCase struct {
	events   []domain.AuditEvent
	segments map[int64][]domain.AuditOutputSegment
	err      error
}

func (f *fakeAuditUseCase) Query(sessionID string) ([]domain.AuditEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
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

func (f *fakeAuditUseCase) LoadOutputSegments(auditID int64) ([]domain.AuditOutputSegment, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.segments[auditID], nil
}

// TestAuditService_Query_MapsAllFields confirms every domain.AuditEvent
// field (including the flattened ResolveSummary) survives the DTO
// conversion, and that Timestamp is converted to Unix seconds.
func TestAuditService_Query_MapsAllFields(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0)
	uc := &fakeAuditUseCase{events: []domain.AuditEvent{{
		ID:          7,
		Timestamp:   ts,
		ClientID:    "client-1",
		SessionID:   "s1",
		Kind:        domain.AuditKindCommand,
		Target:      "",
		OriginalCmd: "rm -rf /tmp/x",
		GuardedCmd:  "rm -rf /tmp/x",
		Resolve:     domain.ResolveSummary{Risk: "high", Uncertain: true, Reasons: []string{"destructive"}},
		Approver:    "custodian",
		Decision:    "approved",
	}}}
	svc := NewAuditService(uc)

	dtos, err := svc.Query("s1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(dtos) != 1 {
		t.Fatalf("Query() returned %d dtos, want 1", len(dtos))
	}
	got := dtos[0]
	want := AuditEventDTO{
		ID: 7, Timestamp: ts.Unix(), ClientID: "client-1", SessionID: "s1",
		Kind: "command", OriginalCmd: "rm -rf /tmp/x", GuardedCmd: "rm -rf /tmp/x",
		Risk: "high", Uncertain: true, Reasons: []string{"destructive"},
		Approver: "custodian", Decision: "approved",
	}
	if got.ID != want.ID || got.Timestamp != want.Timestamp || got.ClientID != want.ClientID ||
		got.SessionID != want.SessionID || got.Kind != want.Kind || got.OriginalCmd != want.OriginalCmd ||
		got.GuardedCmd != want.GuardedCmd || got.Risk != want.Risk || got.Uncertain != want.Uncertain ||
		got.Approver != want.Approver || got.Decision != want.Decision || len(got.Reasons) != 1 || got.Reasons[0] != "destructive" {
		t.Fatalf("Query()[0] = %+v, want %+v", got, want)
	}
}

// TestAuditService_Query_PropagatesError confirms a use-case error surfaces
// rather than being swallowed into an empty slice.
func TestAuditService_Query_PropagatesError(t *testing.T) {
	uc := &fakeAuditUseCase{err: errBoom}
	svc := NewAuditService(uc)

	if _, err := svc.Query(""); err != errBoom {
		t.Fatalf("Query() error = %v, want %v", err, errBoom)
	}
}

// TestAuditService_LoadOutput_MapsSegments confirms redacted and plain
// segments both map through, including the per-item-unmask Value.
func TestAuditService_LoadOutput_MapsSegments(t *testing.T) {
	uc := &fakeAuditUseCase{segments: map[int64][]domain.AuditOutputSegment{
		42: {
			{Text: "hello "},
			{Redacted: true, Type: "aws-access-key-id", Value: "AKIAABCDEFGHIJKLMNOP"},
			{Text: " world"},
		},
	}}
	svc := NewAuditService(uc)

	dtos, err := svc.LoadOutput(42)
	if err != nil {
		t.Fatalf("LoadOutput() error = %v", err)
	}
	if len(dtos) != 3 {
		t.Fatalf("LoadOutput() returned %d segments, want 3", len(dtos))
	}
	if dtos[0].Redacted || dtos[0].Text != "hello " {
		t.Fatalf("segments[0] = %+v, want plain %q", dtos[0], "hello ")
	}
	if !dtos[1].Redacted || dtos[1].Type != "aws-access-key-id" || dtos[1].Value != "AKIAABCDEFGHIJKLMNOP" {
		t.Fatalf("segments[1] = %+v, want redacted aws-access-key-id", dtos[1])
	}
	if dtos[2].Redacted || dtos[2].Text != " world" {
		t.Fatalf("segments[2] = %+v, want plain %q", dtos[2], " world")
	}
}

// TestAuditService_LoadOutput_NoneAttachedReturnsEmpty confirms an
// unattached/expired capture maps to an empty slice, not a nil-vs-error
// surprise for the frontend.
func TestAuditService_LoadOutput_NoneAttachedReturnsEmpty(t *testing.T) {
	svc := NewAuditService(&fakeAuditUseCase{})

	dtos, err := svc.LoadOutput(99)
	if err != nil {
		t.Fatalf("LoadOutput() error = %v", err)
	}
	if len(dtos) != 0 {
		t.Fatalf("LoadOutput() = %+v, want empty", dtos)
	}
}

var errBoom = &boomErr{}

type boomErr struct{}

func (*boomErr) Error() string { return "boom" }
