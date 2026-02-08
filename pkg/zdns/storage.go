package zdns

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Storage handles persisting the PeerStore and Secrets to disk.
type Storage struct {
	ConfigDir string
	Secrets   SecretStore
	vault     *Vault
}

func NewStorage(password ...string) (*Storage, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	zdnsDir := filepath.Join(configDir, "zdns")
	if err := os.MkdirAll(zdnsDir, 0700); err != nil {
		return nil, err
	}

	var vault *Vault
	if len(password) > 0 && password[0] != "" {
		salt, err := GetOrCreateSalt(filepath.Join(zdnsDir, "vault.salt"))
		if err != nil {
			return nil, err
		}
		vault = NewVault(password[0], salt)
	}

	secrets, err := NewFileSecretStore(zdnsDir, vault)
	if err != nil {
		return nil, err
	}

	return &Storage{
		ConfigDir: zdnsDir,
		Secrets:   secrets,
		vault:     vault,
	}, nil
}

func (s *Storage) GetPeersPath() string {
	return filepath.Join(s.ConfigDir, "peers.json")
}

func (s *Storage) GetVault() *Vault {
	return s.vault
}

// SavePeers serializes the peers metadata and their secrets separately.
func (s *Storage) SavePeers(peers []*Peer) error {
	// 1. Save Secrets via SecretStore
	for _, p := range peers {
		if len(p.SharedSecret) > 0 {
			if err := s.Secrets.SetSecret(p.PublicKey, p.SharedSecret); err != nil {
				return err
			}
		}
	}

	// 2. Save Metadata
	data, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return err
	}

	if s.vault != nil {
		data, err = s.vault.Encrypt(data)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(s.GetPeersPath(), data, 0600)
}

// LoadPeers reads peers and reconstitutes them with their secrets.
func (s *Storage) LoadPeers() ([]*Peer, error) {
	path := s.GetPeersPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []*Peer{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if s.vault != nil {
		data, err = s.vault.Decrypt(data)
		if err != nil {
			return nil, err
		}
	}

	var peers []*Peer
	if err := json.Unmarshal(data, &peers); err != nil {
		return nil, err
	}

	// Reconstitute secrets
	for _, p := range peers {
		secret, err := s.Secrets.GetSecret(p.PublicKey)
		if err == nil {
			p.SharedSecret = secret
		}
	}

	return peers, nil
}
