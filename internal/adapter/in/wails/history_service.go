package wails

import (
	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/in"
)

// localScope is the sentinel HostID value meaning "local sessions only" on
// the wire -- real host IDs are UUIDs (uuid.NewString()) and never collide
// with this literal. "" means no filter (every session).
const localScope = "local"

// HistoryQueryDTO is the JSON-facing request DTO for ListHistory. HostID is
// "" for every session, "local" for local shells only, or a specific host ID.
type HistoryQueryDTO struct {
	HostID string `json:"hostId"`
	Search string `json:"search"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// HistoryEntryDTO is the JSON-facing response DTO for a history entry.
type HistoryEntryDTO struct {
	ID         int64  `json:"id"`
	HostID     string `json:"hostId,omitempty"`
	Command    string `json:"command"`
	ExecutedAt int64  `json:"executedAt"`
}

// HistoryService is the Wails-bound facade over in.HistoryUseCase. It only
// converts between JSON-facing DTOs and domain types -- no business logic.
type HistoryService struct {
	uc in.HistoryUseCase
}

func NewHistoryService(uc in.HistoryUseCase) *HistoryService {
	return &HistoryService{uc: uc}
}

func (s *HistoryService) ListHistory(q HistoryQueryDTO) ([]HistoryEntryDTO, error) {
	entries, err := s.uc.List(domain.HistoryQuery{
		HostID: hostIDFilter(q.HostID),
		Search: q.Search,
		Limit:  q.Limit,
		Offset: q.Offset,
	})
	if err != nil {
		return nil, err
	}
	dtos := make([]HistoryEntryDTO, len(entries))
	for i, e := range entries {
		dtos[i] = historyEntryToDTO(e)
	}
	return dtos, nil
}

func (s *HistoryService) DeleteHistoryEntry(id int64) error {
	return s.uc.Delete(id)
}

// ClearHistory deletes local entries ("local"), one host's entries (a host
// ID), or every entry ("").
func (s *HistoryService) ClearHistory(hostID string) error {
	return s.uc.Clear(hostIDFilter(hostID))
}

func hostIDFilter(wire string) *string {
	switch wire {
	case "":
		return nil
	case localScope:
		local := ""
		return &local
	default:
		return &wire
	}
}

func historyEntryToDTO(e domain.HistoryEntry) HistoryEntryDTO {
	dto := HistoryEntryDTO{ID: e.ID, Command: e.Command, ExecutedAt: e.ExecutedAt}
	if e.HostID != nil {
		dto.HostID = *e.HostID
	}
	return dto
}
