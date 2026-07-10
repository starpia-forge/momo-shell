package audit

import (
	"errors"
	"sync"
)

var errNotFound = errors.New("audit test: secret not found")

// fakeSecretStore is a hand-rolled in-memory out.SecretStore (house style:
// no mock framework -- see host/mocks_test.go's fakeSecretStore mirror),
// shared by audit_test.go and cipher_test.go.
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
