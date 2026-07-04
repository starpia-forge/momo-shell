package host

import (
	"errors"
	"sync"

	"momo-terminal/internal/core/domain"
	"momo-terminal/internal/core/port/out"
)

var errNotFound = errors.New("host: not found")

// fakeRepo is an in-memory out.HostRepository for tests.
type fakeRepo struct {
	mu    sync.Mutex
	hosts map[string]domain.Host

	saveErr   error
	deleteErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{hosts: make(map[string]domain.Host)}
}

func (r *fakeRepo) List() ([]domain.Host, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Host, 0, len(r.hosts))
	for _, h := range r.hosts {
		out = append(out, h)
	}
	return out, nil
}

func (r *fakeRepo) Get(id string) (domain.Host, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hosts[id]
	if !ok {
		return domain.Host{}, errNotFound
	}
	return h, nil
}

func (r *fakeRepo) Save(h domain.Host) (domain.Host, error) {
	if r.saveErr != nil {
		return domain.Host{}, r.saveErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hosts[h.ID] = h
	return h, nil
}

func (r *fakeRepo) Delete(id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.hosts[id]; !ok {
		return errNotFound
	}
	delete(r.hosts, id)
	return nil
}

func (r *fakeRepo) TouchConnected(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.hosts[id]; !ok {
		return errNotFound
	}
	return nil
}

func (r *fakeRepo) ListLabels() ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[string]bool{}
	var out []string
	for _, h := range r.hosts {
		for _, l := range h.Labels {
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	}
	return out, nil
}

// fakeSecretStore is an in-memory out.SecretStore for tests.
type fakeSecretStore struct {
	mu      sync.Mutex
	secrets map[string][]byte
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{secrets: make(map[string][]byte)}
}

func (s *fakeSecretStore) Set(ref string, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[ref] = secret
	return nil
}

func (s *fakeSecretStore) Get(ref string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.secrets[ref]
	if !ok {
		return nil, errNotFound
	}
	return v, nil
}

func (s *fakeSecretStore) Delete(ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, ref)
	return nil
}

func (s *fakeSecretStore) has(ref string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.secrets[ref]
	return ok
}

// fakeProber is an out.SSHProber stub that returns whatever result was
// configured and records the last call's arguments.
type fakeProber struct {
	result out.TestResult

	lastHost   domain.Host
	lastSecret string
}

func (p *fakeProber) Probe(h domain.Host, secret string) out.TestResult {
	p.lastHost = h
	p.lastSecret = secret
	return p.result
}
