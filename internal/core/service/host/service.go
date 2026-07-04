package host

import (
	"time"

	"github.com/google/uuid"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/in"
	"momo-terminal/internal/core/port/out"
)

// Service implements in.HostUseCase, delegating storage to out.HostRepository
// and secret material to out.SecretStore.
type Service struct {
	repo   out.HostRepository
	secret out.SecretStore
	prober out.SSHProber
}

var _ in.HostUseCase = (*Service)(nil)

func New(repo out.HostRepository, secret out.SecretStore, prober out.SSHProber) *Service {
	return &Service{repo: repo, secret: secret, prober: prober}
}

func secretRef(id string) string {
	return "host:" + id
}

func (s *Service) ListHosts() ([]domain.Host, error) {
	return s.repo.List()
}

func (s *Service) GetHost(id string) (domain.Host, error) {
	return s.repo.Get(id)
}

// SaveHost creates a host when in.ID == "", otherwise loads and updates the
// existing record so CreatedAt is preserved across edits.
func (s *Service) SaveHost(input in.HostInput) (domain.Host, error) {
	now := time.Now()

	h := domain.Host{ID: input.ID, CreatedAt: now}
	if input.ID != "" {
		existing, err := s.repo.Get(input.ID)
		if err != nil {
			return domain.Host{}, err
		}
		h = existing
	} else {
		h.ID = uuid.NewString()
	}

	h.Name = input.Name
	h.Address = input.Address
	h.Port = input.Port
	h.Labels = input.Labels
	h.Username = input.Username
	h.AuthType = input.AuthType
	h.KeyPath = input.KeyPath
	h.UpdatedAt = now

	if err := h.Validate(); err != nil {
		return domain.Host{}, err
	}

	return s.repo.Save(h)
}

func (s *Service) DeleteHost(id string) error {
	_ = s.secret.Delete(secretRef(id))
	return s.repo.Delete(id)
}

func (s *Service) SetHostSecret(id string, secret string) error {
	if _, err := s.repo.Get(id); err != nil {
		return err
	}
	return s.secret.Set(secretRef(id), []byte(secret))
}

func (s *Service) TestConnection(id string) (in.TestResult, error) {
	h, err := s.repo.Get(id)
	if err != nil {
		return in.TestResult{}, err
	}

	var secretVal string
	if h.AuthType != domain.AuthAgent {
		if b, err := s.secret.Get(secretRef(id)); err == nil {
			secretVal = string(b)
		}
	}

	result := s.prober.Probe(h, secretVal)
	return in.TestResult{Stage: string(result.Stage), OK: result.OK, Message: result.Message}, nil
}

func (s *Service) ListLabels() ([]string, error) {
	return s.repo.ListLabels()
}
