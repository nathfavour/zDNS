package zdns

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Storage handles persisting the PeerStore to disk.
type Storage struct {
	ConfigDir string
}

func NewStorage() (*Storage, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	zdnsDir := filepath.Join(configDir, "zdns")
	// Ensure directory exists with strict permissions (0700)
	if err := os.MkdirAll(zdnsDir, 0700); err != nil {
		return nil, err
	}

	return &Storage{ConfigDir: zdnsDir}, nil
}

func (s *Storage) GetPeersPath() string {
	return filepath.Join(s.ConfigDir, "peers.json")
}

// SavePeers serializes the peers to disk.
func (s *Storage) SavePeers(peers []*Peer) error {
	data, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return err
	}

	// Write with 0600 permissions (read/write for owner only)
	return os.WriteFile(s.GetPeersPath(), data, 0600)
}

// LoadPeers reads peers from disk.
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

	return peers, nil
}
