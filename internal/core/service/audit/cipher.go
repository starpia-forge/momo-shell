package audit

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	"momo-shell/internal/core/port/out"
)

// outputKeyRef is the fixed out.SecretStore ref under which E3-b's
// AES-256-GCM output-encryption key lives -- one local key for every
// captured command's output blob, distinct from per-host credential refs
// ("host:{id}"). SecretStore holds only the 32-byte key; the ciphertext
// blobs themselves live in mcp_audit.output_ref (too large for the OS
// keychain's small-blob backends).
const outputKeyRef = "audit:output-key"

// cipherBox loads-or-creates the output-encryption key exactly once and
// caches the derived AEAD (fallback_file.go's AES-GCM shape, keyed via
// SecretStore instead of a sibling file). Doing this lazily but only once
// matters: capture finalization runs from goroutines spawned per command,
// and two concurrent Get-misses racing to two different Set calls would
// leave blobs sealed under the loser's key permanently unreadable.
type cipherBox struct {
	secrets out.SecretStore

	once sync.Once
	aead cipher.AEAD
	err  error
}

func newCipherBox(secrets out.SecretStore) *cipherBox {
	return &cipherBox{secrets: secrets}
}

func (c *cipherBox) get() (cipher.AEAD, error) {
	c.once.Do(func() {
		c.aead, c.err = c.loadOrCreate()
	})
	return c.aead, c.err
}

// loadOrCreate mirrors host/service.go's secret.Get idiom: any Get error
// (unknown ref or otherwise) is treated as "no key yet" and a fresh one is
// generated -- out.SecretStore's port doesn't export a typed not-found
// error for callers outside the keychain adapter to check.
func (c *cipherBox) loadOrCreate() (cipher.AEAD, error) {
	key, err := c.secrets.Get(outputKeyRef)
	if err != nil {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("audit: generate output key: %w", err)
		}
		if err := c.secrets.Set(outputKeyRef, key); err != nil {
			return nil, fmt.Errorf("audit: store output key: %w", err)
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("audit: init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("audit: init gcm: %w", err)
	}
	return gcm, nil
}

// seal encrypts plaintext with a fresh random nonce, prepended to the
// returned ciphertext (fallback_file.go's format).
func (c *cipherBox) seal(plaintext []byte) ([]byte, error) {
	gcm, err := c.get()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("audit: generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// open decrypts a blob produced by seal.
func (c *cipherBox) open(ciphertext []byte) ([]byte, error) {
	gcm, err := c.get()
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("audit: corrupt output capture")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, sealed, nil)
}
