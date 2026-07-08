package domain

// HostRef is the AI-facing projection of a saved Host: name and labels
// only. Address/username/auth/key/secret are excluded by construction --
// the AI says connect_host(name); the custodian fills credentials
// (design doc 16 §핵심원칙①). This is even narrower than SharedHost, which
// still carries Address/Port/Username for a trusted LAN peer.
type HostRef struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
}
