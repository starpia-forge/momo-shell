package domain

// HistoryEntry is one captured command line. HostID is nil for a local
// shell session, or set to the originating host's ID for SSH.
type HistoryEntry struct {
	ID         int64
	HostID     *string
	Command    string
	ExecutedAt int64 // Unix seconds
}

// HistoryQuery filters HistoryRepository.List. HostID has three states,
// same as HistoryRepository.Clear: nil means every session (local and every
// host); a pointer to "" means local sessions only; a pointer to a host ID
// scopes to that host. (This differs from Append/LastForHost, where nil
// alone means local -- those never need an "every session" state.)
// Limit <= 0 means "no limit".
type HistoryQuery struct {
	HostID *string
	Search string
	Limit  int
	Offset int
}
