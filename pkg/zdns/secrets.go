package zdns

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SecretStore defines how shared secrets are persisted.
// This allows swapping between file-based storage and OS keyrings.
type SecretStore interface {
	GetSecret(deviceID [32]byte) ([]byte, error)
	SetSecret(deviceID [32]byte, secret []byte) error
	RemoveSecret(deviceID [32]byte) error
}

// FileSecretStore implements SecretStore using a dedicated file.
type FileSecretStore struct {
	mu    sync.RWMutex
	path  string
	data  map[string][]byte
	vault *Vault
}

func (s *FileSecretStore) RemoveSecret(deviceID [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%x", deviceID)
	delete(s.data, key)
	return s.save()
}

func NewFileSecretStore(dir string, vault ...*Vault) (*FileSecretStore, error) {
	path := filepath.Join(dir, "secrets.json")
	store := &FileSecretStore{
		path: path,
		data: make(map[string][]byte),
	}
	if len(vault) > 0 {
		store.vault = vault[0]
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *FileSecretStore) load() error {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	if s.vault != nil {
		data, err = s.vault.Decrypt(data)
		if err != nil {
			return err
		}
	}

	return json.Unmarshal(data, &s.data)
}

func (s *FileSecretStore) save() error {
	data, err := json.Marshal(s.data)
	if err != nil {
		return err
	}

	if s.vault != nil {
		data, err = s.vault.Encrypt(data)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(s.path, data, 0600)
}

func (s *FileSecretStore) GetSecret(deviceID [32]byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := fmt.Sprintf("%x", deviceID)
	secret, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("secret not found for device %s", key)
	}
	return secret, nil
}

func (s *FileSecretStore) SetSecret(deviceID [32]byte, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%x", deviceID)
	s.data[key] = secret
	return s.save()
}
