// Package keychain implements the SecretStore out-port using the OS
// keychain (via go-keyring), falling back to an AES-GCM encrypted file on
// platforms where no keychain is available (some headless Linux setups).
package keychain

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"

	"momo-shell/internal/core/port/out"
)

const serviceName = "momo-shell"

// ErrSecretNotFound is returned by Get for an unknown ref.
var ErrSecretNotFound = errors.New("keychain: secret not found")

// Store implements out.SecretStore. Secrets are base64-encoded before
// being handed to the keyring/file backend so arbitrary binary content
// survives round-trips through backends that only guarantee string storage.
type Store struct {
	fallback *fileStore
}

var _ out.SecretStore = (*Store)(nil)

// New probes the OS keychain and returns a Store backed by it, or by an
// encrypted fallback file if the keychain is unavailable.
func New() (*Store, error) {
	if keyringAvailable() {
		return &Store{}, nil
	}
	fb, err := newFileStore()
	if err != nil {
		return nil, err
	}
	return &Store{fallback: fb}, nil
}

func keyringAvailable() bool {
	const probeUser = "__momo_probe__"
	if err := keyring.Set(serviceName, probeUser, "probe"); err != nil {
		return false
	}
	_ = keyring.Delete(serviceName, probeUser)
	return true
}

func (s *Store) Set(ref string, secret []byte) error {
	encoded := base64.StdEncoding.EncodeToString(secret)
	if s.fallback != nil {
		return s.fallback.Set(ref, encoded)
	}
	if err := keyring.Set(serviceName, ref, encoded); err != nil {
		return fmt.Errorf("keychain: set %s: %w", ref, err)
	}
	return nil
}

func (s *Store) Get(ref string) ([]byte, error) {
	var encoded string
	if s.fallback != nil {
		v, err := s.fallback.Get(ref)
		if err != nil {
			return nil, err
		}
		encoded = v
	} else {
		v, err := keyring.Get(serviceName, ref)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrSecretNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("keychain: get %s: %w", ref, err)
		}
		encoded = v
	}
	return base64.StdEncoding.DecodeString(encoded)
}

func (s *Store) Delete(ref string) error {
	if s.fallback != nil {
		return s.fallback.Delete(ref)
	}
	if err := keyring.Delete(serviceName, ref); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("keychain: delete %s: %w", ref, err)
	}
	return nil
}
