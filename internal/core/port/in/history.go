package in

import "momo-shell/internal/core/domain"

// HistoryUseCase is the driving port for browsing and managing captured
// command history. Capture itself isn't part of this interface -- it's
// wired directly into the session service via session.CommandTap.
type HistoryUseCase interface {
	List(q domain.HistoryQuery) ([]domain.HistoryEntry, error)
	Delete(id int64) error
	// Clear deletes entries for hostID, or every entry when hostID is nil.
	Clear(hostID *string) error
}
