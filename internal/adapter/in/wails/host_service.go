package wails

import (
	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/in"
)

// HostDTO is the JSON-facing response DTO for a saved host. Secret material
// (password/key passphrase) never appears here -- see SetHostSecret.
type HostDTO struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Address         string   `json:"address"`
	Port            int      `json:"port"`
	Labels          []string `json:"labels"`
	Username        string   `json:"username"`
	AuthType        string   `json:"authType"`
	KeyPath         string   `json:"keyPath,omitempty"`
	Source          string   `json:"source"`
	CreatedAt       int64    `json:"createdAt"`
	UpdatedAt       int64    `json:"updatedAt"`
	LastConnectedAt *int64   `json:"lastConnectedAt,omitempty"`
}

// HostInputDTO is the JSON-facing request DTO for SaveHost. ID == "" creates
// a new host; a non-empty ID updates the existing one.
type HostInputDTO struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Labels   []string `json:"labels"`
	Username string   `json:"username"`
	AuthType string   `json:"authType"`
	KeyPath  string   `json:"keyPath"`
}

// TestResultDTO is the JSON-facing response DTO for TestConnection.
type TestResultDTO struct {
	Stage   string `json:"stage"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// HostService is the Wails-bound facade over in.HostUseCase. It only
// converts between JSON-facing DTOs and domain types -- no business logic.
type HostService struct {
	uc in.HostUseCase
}

func NewHostService(uc in.HostUseCase) *HostService {
	return &HostService{uc: uc}
}

func (s *HostService) ListHosts() ([]HostDTO, error) {
	hosts, err := s.uc.ListHosts()
	if err != nil {
		return nil, err
	}
	dtos := make([]HostDTO, len(hosts))
	for i, h := range hosts {
		dtos[i] = hostToDTO(h)
	}
	return dtos, nil
}

func (s *HostService) GetHost(id string) (HostDTO, error) {
	h, err := s.uc.GetHost(id)
	if err != nil {
		return HostDTO{}, err
	}
	return hostToDTO(h), nil
}

func (s *HostService) SaveHost(input HostInputDTO) (HostDTO, error) {
	h, err := s.uc.SaveHost(in.HostInput{
		ID:       input.ID,
		Name:     input.Name,
		Address:  input.Address,
		Port:     input.Port,
		Labels:   input.Labels,
		Username: input.Username,
		AuthType: domain.AuthType(input.AuthType),
		KeyPath:  input.KeyPath,
	})
	if err != nil {
		return HostDTO{}, err
	}
	return hostToDTO(h), nil
}

func (s *HostService) DeleteHost(id string) error {
	return s.uc.DeleteHost(id)
}

func (s *HostService) SetHostSecret(id string, secret string) error {
	return s.uc.SetHostSecret(id, secret)
}

func (s *HostService) TestConnection(id string) (TestResultDTO, error) {
	result, err := s.uc.TestConnection(id)
	if err != nil {
		return TestResultDTO{}, err
	}
	return TestResultDTO{Stage: result.Stage, OK: result.OK, Message: result.Message}, nil
}

func (s *HostService) ListLabels() ([]string, error) {
	return s.uc.ListLabels()
}

func hostToDTO(h domain.Host) HostDTO {
	dto := HostDTO{
		ID:        h.ID,
		Name:      h.Name,
		Address:   h.Address,
		Port:      h.Port,
		Labels:    h.Labels,
		Username:  h.Username,
		AuthType:  string(h.AuthType),
		KeyPath:   h.KeyPath,
		Source:    h.Source,
		CreatedAt: h.CreatedAt.Unix(),
		UpdatedAt: h.UpdatedAt.Unix(),
	}
	if h.LastConnectedAt != nil {
		t := h.LastConnectedAt.Unix()
		dto.LastConnectedAt = &t
	}
	return dto
}
