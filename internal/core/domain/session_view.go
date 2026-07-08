package domain

// SessionView is the AI-facing projection of a Session: unlike SessionInfo
// (a stable identity/geometry snapshot), it surfaces the mutable lifecycle
// State and whether an AI client currently holds control delegation over
// it (design doc 17 §4 "상태+AI통제여부").
type SessionView struct {
	ID         string       `json:"id"`
	Kind       SessionKind  `json:"kind"`
	HostID     string       `json:"hostId,omitempty"`
	State      SessionState `json:"state"`
	Controlled bool         `json:"controlled"`
}
