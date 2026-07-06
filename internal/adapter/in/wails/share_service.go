package wails

import (
	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
)

// ShareStatusDTO is the JSON-facing response DTO for the share settings panel.
type ShareStatusDTO struct {
	Enabled       bool     `json:"enabled"`
	PIN           string   `json:"pin"`
	Port          int      `json:"port"`
	InstanceName  string   `json:"instanceName"`
	SharedHostIDs []string `json:"sharedHostIds"`
}

// ShareClientDTO is the JSON-facing response DTO for a paired provider-side client.
type ShareClientDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PairedAt   int64  `json:"pairedAt"`
	LastSeenAt *int64 `json:"lastSeenAt,omitempty"`
}

// SharedHostDTO mirrors domain.SharedHost, which already excludes secret
// and structural fields (id, authType, keyPath) by construction.
type SharedHostDTO struct {
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Labels   []string `json:"labels"`
	Username string   `json:"username"`
}

// PeerViewDTO is the JSON-facing snapshot of one peer shown in the host
// sidebar's shared-hosts section.
type PeerViewDTO struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Address    string          `json:"address"`
	Port       int             `json:"port"`
	Paired     bool            `json:"paired"`
	Online     bool            `json:"online"`
	LastSyncAt *int64          `json:"lastSyncAt,omitempty"`
	Hosts      []SharedHostDTO `json:"hosts"`
}

// ShareService is the Wails-bound facade over in.ShareUseCase. It only
// converts between JSON-facing DTOs and domain types -- no business logic.
type ShareService struct {
	uc in.ShareUseCase
}

func NewShareService(uc in.ShareUseCase) *ShareService {
	return &ShareService{uc: uc}
}

func (s *ShareService) EnableSharing(hostIDs []string) (ShareStatusDTO, error) {
	status, err := s.uc.EnableSharing(hostIDs)
	if err != nil {
		return ShareStatusDTO{}, err
	}
	return statusToDTO(status), nil
}

func (s *ShareService) DisableSharing() error {
	return s.uc.DisableSharing()
}

func (s *ShareService) Status() (ShareStatusDTO, error) {
	status, err := s.uc.Status()
	if err != nil {
		return ShareStatusDTO{}, err
	}
	return statusToDTO(status), nil
}

func (s *ShareService) SetSharedHosts(hostIDs []string) error {
	return s.uc.SetSharedHosts(hostIDs)
}

func (s *ShareService) ListClients() ([]ShareClientDTO, error) {
	clients, err := s.uc.ListClients()
	if err != nil {
		return nil, err
	}
	dtos := make([]ShareClientDTO, len(clients))
	for i, c := range clients {
		dtos[i] = clientToDTO(c)
	}
	return dtos, nil
}

func (s *ShareService) RevokeClient(clientID string) error {
	return s.uc.RevokeClient(clientID)
}

func (s *ShareService) RespondPairing(requestID string, approve bool) error {
	return s.uc.RespondPairing(requestID, approve)
}

func (s *ShareService) ListPeers() ([]PeerViewDTO, error) {
	peers, err := s.uc.ListPeers()
	if err != nil {
		return nil, err
	}
	dtos := make([]PeerViewDTO, len(peers))
	for i, p := range peers {
		dtos[i] = peerViewToDTO(p)
	}
	return dtos, nil
}

func (s *ShareService) PairWithPeer(peerID string, pin string) error {
	return s.uc.PairWithPeer(peerID, pin)
}

func (s *ShareService) AddPeerByAddress(address string, port int, pin string) error {
	return s.uc.AddPeerByAddress(address, port, pin)
}

func (s *ShareService) RemovePeer(peerID string) error {
	return s.uc.RemovePeer(peerID)
}

func (s *ShareService) FetchSharedHosts(peerID string) ([]SharedHostDTO, error) {
	hosts, err := s.uc.FetchSharedHosts(peerID)
	if err != nil {
		return nil, err
	}
	dtos := make([]SharedHostDTO, len(hosts))
	for i, h := range hosts {
		dtos[i] = sharedHostToDTO(h)
	}
	return dtos, nil
}

func (s *ShareService) ImportSharedHost(peerID string, index int) (HostDTO, error) {
	host, err := s.uc.ImportSharedHost(peerID, index)
	if err != nil {
		return HostDTO{}, err
	}
	return hostToDTO(host), nil
}

func (s *ShareService) DeviceName() (string, error) {
	return s.uc.DeviceName()
}

func (s *ShareService) SetDeviceName(name string) error {
	return s.uc.SetDeviceName(name)
}

func statusToDTO(status in.ShareStatus) ShareStatusDTO {
	return ShareStatusDTO{
		Enabled:       status.Enabled,
		PIN:           status.PIN,
		Port:          status.Port,
		InstanceName:  status.InstanceName,
		SharedHostIDs: status.SharedHostIDs,
	}
}

func clientToDTO(c domain.ShareClient) ShareClientDTO {
	dto := ShareClientDTO{ID: c.ID, Name: c.Name, PairedAt: c.PairedAt.Unix()}
	if c.LastSeenAt != nil {
		t := c.LastSeenAt.Unix()
		dto.LastSeenAt = &t
	}
	return dto
}

func peerViewToDTO(p in.PeerView) PeerViewDTO {
	dto := PeerViewDTO{
		ID:      p.ID,
		Name:    p.Name,
		Address: p.Address,
		Port:    p.Port,
		Paired:  p.Paired,
		Online:  p.Online,
		Hosts:   make([]SharedHostDTO, len(p.Hosts)),
	}
	if p.LastSyncAt != nil {
		t := p.LastSyncAt.Unix()
		dto.LastSyncAt = &t
	}
	for i, h := range p.Hosts {
		dto.Hosts[i] = sharedHostToDTO(h)
	}
	return dto
}

func sharedHostToDTO(h domain.SharedHost) SharedHostDTO {
	return SharedHostDTO{Name: h.Name, Address: h.Address, Port: h.Port, Labels: h.Labels, Username: h.Username}
}
