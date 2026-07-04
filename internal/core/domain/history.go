package domain

// HistoryEntry is one captured command line. HostID is nil for a local
// shell session, or set to the originating host's ID for SSH.
type HistoryEntry struct {
	ID         int64
	HostID     *string
	Command    string
	ExecutedAt int64 // Unix seconds
}

// HistoryQuery filters HistoryRepository.List. HostID nil means "every
// session, local and SSH alike"; Limit <= 0 means "no limit".
type HistoryQuery struct {
	HostID *string
	Search string
	Limit  int
	Offset int
}
