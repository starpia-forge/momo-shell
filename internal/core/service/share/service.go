// Package share implements in.ShareUseCase (the local Wails UI's provider
// controls) and in.ShareServerCallbacks (the LAN HTTPS server's callback
// into core for pairing and host-list requests). Consumer-side behavior
// (discovering and pairing with peers) is added in M2.
package share

import (
	"context"
	"os"
	"sync"
	"time"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

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

// Deps are the out-ports/collaborators Service needs.
type Deps struct {
	HostRepo out.HostRepository
	Clients  out.ShareClientRepository
	Settings out.ShareSettings
	Pub      out.EventPublisher
}

// Service implements in.ShareUseCase and in.ShareServerCallbacks. The LAN
// HTTPS server (out.ShareServer) is wired in after construction via
// SetServer -- see SetServer for why.
type Service struct {
	hostRepo out.HostRepository
	clients  out.ShareClientRepository
	settings out.ShareSettings
	pub      out.EventPublisher
	server   out.ShareServer

	mu          sync.Mutex
	enabled     bool
	pin         string
	port        int
	pinFailures int
	lockedUntil time.Time
	pending     map[string]chan bool
}

var _ in.ShareUseCase = (*Service)(nil)
var _ in.ShareServerCallbacks = (*Service)(nil)

func New(deps Deps) *Service {
	return &Service{
		hostRepo: deps.HostRepo,
		clients:  deps.Clients,
		settings: deps.Settings,
		pub:      deps.Pub,
		pending:  make(map[string]chan bool),
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

	return s.server.Stop(context.Background())
}

func (s *Service) Status() (in.ShareStatus, error) {
	hostIDs, err := s.settings.SharedHostIDs()
	if err != nil {
		return in.ShareStatus{}, err
	}
	name, _ := os.Hostname()

	s.mu.Lock()
	defer s.mu.Unlock()
	return in.ShareStatus{
		Enabled:       s.enabled,
		PIN:           s.pin,
		Port:          s.port,
		InstanceName:  name,
		SharedHostIDs: hostIDs,
	}, nil
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
