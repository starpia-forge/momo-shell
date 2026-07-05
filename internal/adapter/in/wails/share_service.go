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
