package domain

import "time"

// SharedHost is the metadata shared with a paired peer over the LAN share
// API (docs/plan/06-phase5-host-sharing.md §2.2). It deliberately excludes
// secret and structural fields (id, authType, keyPath) -- credentials
// never cross the wire, by construction rather than by filtering.
type SharedHost struct {
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Labels   []string `json:"labels"`
	Username string   `json:"username"`
}

// ShareClient is a peer that has paired with this instance in the provider
// role, i.e. this instance issued them a bearer token. The token itself is
// never stored here -- see core/service/share.
type ShareClient struct {
	ID         string
	Name       string
	PairedAt   time.Time
	LastSeenAt *time.Time
}
