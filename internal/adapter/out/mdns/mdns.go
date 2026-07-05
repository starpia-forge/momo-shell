// Package mdns implements out.PeerAnnouncer and out.PeerBrowser over
// mDNS/DNS-SD (docs/plan/06-phase5-host-sharing.md §2.1), using
// libp2p/zeroconf/v2 -- a maintained fork of the originally-suggested
// grandcat/zeroconf with an API-compatible Register/Browse surface.
package mdns

const (
	serviceType = "_momo-share._tcp"
	domain      = "local."
)
