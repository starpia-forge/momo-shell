// Package share implements in.ShareUseCase (the local Wails UI's provider
// controls) and in.ShareServerCallbacks (the LAN HTTPS server's callback
// into core for pairing and host-list requests). Consumer-side behavior
// (discovering and pairing with peers) is added in M2.
package share

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// maxDeviceNameBytes mirrors the mDNS instance-name label length limit.
const maxDeviceNameBytes = 63

const (
	maxPinFailures      = 5
	pairLockoutDuration = 60 * time.Second
)

// pairApprovalTimeout bounds how long HandlePair waits for the local
// user's RespondPairing decision. Var (not const) so tests can shrink it
// rather than waiting the full 60s.
var pairApprovalTimeout = 60 * time.Second

// pairRequestPayload is published on share:pair-request.
type pairRequestPayload struct {
	RequestID  string `json:"requestId"`
	ClientName string `json:"clientName"`
	RemoteAddr string `json:"remoteAddr"`
}

// pairRequestResolvedPayload is published on share:pair-request-resolved
// once a request raised via awaitPairApproval is no longer pending
// (answered, timed out, or the requester disconnected) -- lets the approval
// dialog dismiss itself even if RespondPairing was never called for it.
type pairRequestResolvedPayload struct {
	RequestID string `json:"requestId"`
}

// Deps are the out-ports/collaborators Service needs.
type Deps struct {
	HostRepo   out.HostRepository
	Clients    out.ShareClientRepository
	Settings   out.ShareSettings
	Pub        out.EventPublisher
	Peers      out.PeerRepository
	Secrets    out.SecretStore
	Announcer  out.PeerAnnouncer
	Browser    out.PeerBrowser
	PeerClient out.PeerClient
}

// Service implements in.ShareUseCase and in.ShareServerCallbacks. The LAN
// HTTPS server (out.ShareServer) is wired in after construction via
// SetServer -- see SetServer for why.
type Service struct {
	hostRepo   out.HostRepository
	clients    out.ShareClientRepository
	settings   out.ShareSettings
	pub        out.EventPublisher
	server     out.ShareServer
	peers      out.PeerRepository
	secrets    out.SecretStore
	announcer  out.PeerAnnouncer
	browser    out.PeerBrowser
	peerClient out.PeerClient

	mu          sync.Mutex
	enabled     bool
	pin         string
	port        int
	pinFailures int
	lockedUntil time.Time
	pending     map[string]chan bool

	cmu        sync.Mutex
	discovered map[string]out.DiscoveredPeer
	syncCancel context.CancelFunc
	syncWG     sync.WaitGroup
}

var _ in.ShareUseCase = (*Service)(nil)
var _ in.ShareServerCallbacks = (*Service)(nil)

func New(deps Deps) *Service {
	return &Service{
		hostRepo:   deps.HostRepo,
		clients:    deps.Clients,
		settings:   deps.Settings,
		pub:        deps.Pub,
		peers:      deps.Peers,
		secrets:    deps.Secrets,
		announcer:  deps.Announcer,
		browser:    deps.Browser,
		peerClient: deps.PeerClient,
		pending:    make(map[string]chan bool),
		discovered: make(map[string]out.DiscoveredPeer),
	}
}

// SetServer wires the LAN HTTPS server after construction, breaking the
// Service <-> ShareServer construction cycle: the server needs a
// ShareServerCallbacks pointing back at this Service, and this Service
// needs to Start/Stop the server. Mirrors session.Service.SetMiddleware.
func (s *Service) SetServer(server out.ShareServer) {
	s.server = server
}

func (s *Service) EnableSharing(hostIDs []string) (in.ShareStatus, error) {
	if err := s.settings.SetSharedHostIDs(hostIDs); err != nil {
		return in.ShareStatus{}, err
	}

	pin, err := generatePIN()
	if err != nil {
		return in.ShareStatus{}, err
	}

	s.mu.Lock()
	wasEnabled := s.enabled
	s.pin = pin
	s.pinFailures = 0
	s.lockedUntil = time.Time{}
	s.mu.Unlock()

	if !wasEnabled {
		port, err := s.server.Start()
		if err != nil {
			return in.ShareStatus{}, err
		}

		instanceID, err := s.settings.InstanceID()
		if err != nil {
			_ = s.server.Stop(context.Background())
			return in.ShareStatus{}, err
		}
		if err := s.announcer.Announce(instanceID, s.deviceName(), port); err != nil {
			_ = s.server.Stop(context.Background())
			return in.ShareStatus{}, err
		}

		s.mu.Lock()
		s.enabled = true
		s.port = port
		s.mu.Unlock()
	}

	return s.Status()
}

func (s *Service) DisableSharing() error {
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return nil
	}
	s.enabled = false
	s.pin = ""
	s.mu.Unlock()

	s.announcer.Stop()
	return s.server.Stop(context.Background())
}

func (s *Service) Status() (in.ShareStatus, error) {
	hostIDs, err := s.settings.SharedHostIDs()
	if err != nil {
		return in.ShareStatus{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return in.ShareStatus{
		Enabled:       s.enabled,
		PIN:           s.pin,
		Port:          s.port,
		InstanceName:  s.deviceName(),
		SharedHostIDs: hostIDs,
	}, nil
}

// deviceName returns the effective name this instance presents to peers --
// the custom override if set, otherwise os.Hostname().
func (s *Service) deviceName() string {
	name, err := s.settings.DeviceName()
	if err != nil || name == "" {
		hostname, _ := os.Hostname()
		return hostname
	}
	return name
}

func (s *Service) DeviceName() (string, error) {
	return s.deviceName(), nil
}

// SetDeviceName sets a custom device name override ("" clears it, back to
// os.Hostname()), then re-advertises immediately if sharing is currently
// enabled -- otherwise the new name would only take effect on the next
// EnableSharing call.
func (s *Service) SetDeviceName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) > maxDeviceNameBytes {
		return fmt.Errorf("share: device name exceeds %d bytes", maxDeviceNameBytes)
	}
	if err := s.settings.SetDeviceName(name); err != nil {
		return err
	}

	s.mu.Lock()
	enabled, port := s.enabled, s.port
	s.mu.Unlock()
	if !enabled {
		return nil
	}

	instanceID, err := s.settings.InstanceID()
	if err != nil {
		return err
	}
	s.announcer.Stop()
	return s.announcer.Announce(instanceID, s.deviceName(), port)
}

func (s *Service) SetSharedHosts(hostIDs []string) error {
	return s.settings.SetSharedHostIDs(hostIDs)
}

func (s *Service) ListClients() ([]domain.ShareClient, error) {
	return s.clients.List()
}

func (s *Service) RevokeClient(clientID string) error {
	return s.clients.Delete(clientID)
}
