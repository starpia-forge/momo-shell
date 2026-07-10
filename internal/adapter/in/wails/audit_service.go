package wails

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// AuditEventDTO is the JSON-facing response DTO for one AI-control audit
// decision (E5). ResolveSummary is flattened onto the DTO (Risk/Uncertain/
// Reasons), the same treatment DelegationDTO gives domain.Delegation.
type AuditEventDTO struct {
	ID          int64    `json:"id"`
	Timestamp   int64    `json:"timestamp"`
	ClientID    string   `json:"clientId"`
	SessionID   string   `json:"sessionId"`
	Kind        string   `json:"kind"`
	Target      string   `json:"target,omitempty"`
	OriginalCmd string   `json:"originalCmd,omitempty"`
	GuardedCmd  string   `json:"guardedCmd,omitempty"`
	Risk        string   `json:"risk,omitempty"`
	Uncertain   bool     `json:"uncertain,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
	Approver    string   `json:"approver,omitempty"`
	Decision    string   `json:"decision"`
}

// AuditOutputSegmentDTO is the JSON-facing response DTO for one run of a
// command's E3-b captured output (E5b). A non-redacted run carries only
// Text; a redacted run carries Type and the original Value (the panel's
// per-item unmask reveals Value on click).
type AuditOutputSegmentDTO struct {
	Text     string `json:"text,omitempty"`
	Redacted bool   `json:"redacted"`
	Type     string `json:"type,omitempty"`
	Value    string `json:"value,omitempty"`
}

// AuditService is the Wails-bound facade over in.AuditUseCase -- E5's
// frontend-only audit-panel read path. It only converts between JSON-facing
// DTOs and domain types -- no business logic. Deliberately not part of the
// MCP surface (mcpipc/dispatch.go never references it): the AI has no route
// to this facade at all.
type AuditService struct {
	uc in.AuditUseCase
}

func NewAuditService(uc in.AuditUseCase) *AuditService {
	return &AuditService{uc: uc}
}

// Query returns sessionID's audit trail in replay order, or every session's
// events if sessionID is "".
func (s *AuditService) Query(sessionID string) ([]AuditEventDTO, error) {
	events, err := s.uc.Query(sessionID)
	if err != nil {
		return nil, err
	}
	dtos := make([]AuditEventDTO, len(events))
	for i, e := range events {
		dtos[i] = auditEventToDTO(e)
	}
	return dtos, nil
}

// LoadOutput returns auditID's captured original output, masked by default
// and segmented for per-item unmask (E5b). Empty (not nil) when no output
// was ever captured or retention already expired it.
func (s *AuditService) LoadOutput(auditID int64) ([]AuditOutputSegmentDTO, error) {
	segments, err := s.uc.LoadOutputSegments(auditID)
	if err != nil {
		return nil, err
	}
	dtos := make([]AuditOutputSegmentDTO, len(segments))
	for i, seg := range segments {
		dtos[i] = AuditOutputSegmentDTO{Text: seg.Text, Redacted: seg.Redacted, Type: seg.Type, Value: seg.Value}
	}
	return dtos, nil
}

func auditEventToDTO(e domain.AuditEvent) AuditEventDTO {
	return AuditEventDTO{
		ID:          e.ID,
		Timestamp:   e.Timestamp.Unix(),
		ClientID:    e.ClientID,
		SessionID:   e.SessionID,
		Kind:        string(e.Kind),
		Target:      e.Target,
		OriginalCmd: e.OriginalCmd,
		GuardedCmd:  e.GuardedCmd,
		Risk:        e.Resolve.Risk,
		Uncertain:   e.Resolve.Uncertain,
		Reasons:     e.Resolve.Reasons,
		Approver:    e.Approver,
		Decision:    e.Decision,
	}
}
