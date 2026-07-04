package keychain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// fileStore persists ref->secret pairs in a single AES-GCM encrypted file,
// used when the OS keychain isn't available. The master key is a random
// 32 bytes generated on first use and stored alongside the data file with
// owner-only permissions.
type fileStore struct {
	mu       sync.Mutex
	dataPath string
	keyPath  string
}

func newFileStore() (*fileStore, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("keychain: resolve config dir: %w", err)
	}
	appDir := filepath.Join(dir, "momo-terminal")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("keychain: create app dir: %w", err)
	}
	return &fileStore{
		dataPath: filepath.Join(appDir, "secrets.enc"),
		keyPath:  filepath.Join(appDir, "secrets.key"),
	}, nil
}

func (f *fileStore) Set(ref, encoded string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.load()
	if err != nil {
		return err
	}
	m[ref] = encoded
	return f.save(m)
}

func (f *fileStore) Get(ref string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.load()
	if err != nil {
		return "", err
	}
	v, ok := m[ref]
	if !ok {
		return "", ErrSecretNotFound
	}
	return v, nil
}

func (f *fileStore) Delete(ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.load()
	if err != nil {
		return err
	}
	delete(m, ref)
	return f.save(m)
}

func (f *fileStore) masterKey() ([]byte, error) {
	if key, err := os.ReadFile(f.keyPath); err == nil && len(key) == 32 {
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("keychain: generate master key: %w", err)
	}
	if err := os.WriteFile(f.keyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("keychain: write master key: %w", err)
	}
	return key, nil
}

func (f *fileStore) load() (map[string]string, error) {
	data, err := os.ReadFile(f.dataPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("keychain: read secrets file: %w", err)
	}
	if len(data) == 0 {
		return map[string]string{}, nil
	}

	gcm, err := f.cipher()
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("keychain: corrupt secrets file")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("keychain: decrypt secrets file: %w", err)
	}

	m := map[string]string{}
	if err := json.Unmarshal(plaintext, &m); err != nil {
		return nil, fmt.Errorf("keychain: parse secrets file: %w", err)
	}
	return m, nil
}

func (f *fileStore) save(m map[string]string) error {
	plaintext, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("keychain: marshal secrets: %w", err)
	}

	gcm, err := f.cipher()
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("keychain: generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	if err := os.WriteFile(f.dataPath, ciphertext, 0o600); err != nil {
		return fmt.Errorf("keychain: write secrets file: %w", err)
	}
	return nil
}

func (f *fileStore) cipher() (cipher.AEAD, error) {
	key, err := f.masterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("keychain: init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("keychain: init gcm: %w", err)
	}
	return gcm, nil
}
