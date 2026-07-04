package host

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"momo-shell/internal/core/domain"
	"momo-shell/internal/core/port/in"
	"momo-shell/internal/core/port/out"
)

// Service implements in.HostUseCase, delegating storage to out.HostRepository
// and secret material to out.SecretStore.
type Service struct {
	repo       out.HostRepository
	secret     out.SecretStore
	knownHosts out.KnownHostsRepository
	prober     out.SSHProber
}

var _ in.HostUseCase = (*Service)(nil)

func New(repo out.HostRepository, secret out.SecretStore, knownHosts out.KnownHostsRepository, prober out.SSHProber) *Service {
	return &Service{repo: repo, secret: secret, knownHosts: knownHosts, prober: prober}
}

// nonInteractiveVerifier lets a connection test proceed past both a known,
// matching host key and a never-seen one (there's no user to prompt during
// a quick test) but fails the handshake on a recorded fingerprint mismatch
// -- the one case a test genuinely should catch before saving a host.
func nonInteractiveVerifier(knownHosts out.KnownHostsRepository, address string, port int) out.HostKeyVerifier {
	return func(algo, fingerprint string) (out.HostKeyDecision, error) {
		known, found, err := knownHosts.Get(address, port, algo)
		if err != nil {
			return out.HostKeyCancel, err
		}
		if found && known != fingerprint {
			return out.HostKeyCancel, fmt.Errorf("host key mismatch for %s:%d (%s)", address, port, algo)
		}
		return out.HostKeyOnce, nil
	}
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

	result := s.prober.Probe(h, secretVal, nonInteractiveVerifier(s.knownHosts, h.Address, h.Port))
	return in.TestResult{Stage: string(result.Stage), OK: result.OK, Message: result.Message}, nil
}

func (s *Service) ListLabels() ([]string, error) {
	return s.repo.ListLabels()
}
