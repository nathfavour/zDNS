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
}

func NewStorage() (*Storage, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	zdnsDir := filepath.Join(configDir, "zdns")
	if err := os.MkdirAll(zdnsDir, 0700); err != nil {
		return nil, err
	}

	secrets, err := NewFileSecretStore(zdnsDir)
	if err != nil {
		return nil, err
	}

	return &Storage{
		ConfigDir: zdnsDir,
		Secrets:   secrets,
	}, nil
}

func (s *Storage) GetPeersPath() string {
	return filepath.Join(s.ConfigDir, "peers.json")
}

// SavePeers serializes the peers metadata and their secrets separately.
func (s *Storage) SavePeers(peers []*Peer) error {
	// 1. Save Secrets via SecretStore
	for _, p := range peers {
		if len(p.SharedSecret) > 0 {
			// We use PublicKey as the fingerprint for the device ID in this PoC
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
