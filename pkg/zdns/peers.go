package zdns

import (
	"sync"
	"time"
)

// Peer represents a trusted device.
type Peer struct {
	Name         string   `json:"name"`
	PublicKey    [32]byte `json:"public_key"` // Long-term X25519 Public Key
	ExpiresAt    int64    `json:"expires_at"`
	SharedSecret []byte   `json:"-"`          // Derived Root Secret
}

// PeerStore manages trusted peers.
type PeerStore struct {
	mu    sync.RWMutex
	peers map[[16]byte]*Peer // Keyed by current ServiceID for fast lookup
	all   []*Peer
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[[16]byte]*Peer),
	}
}

func (s *PeerStore) AddPeer(peer *Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = append(s.all, peer)
}

// UpdateServiceIDs recalculates the ServiceID mapping for all peers.
// This should be called periodically or when a packet doesn't match current IDs.
func (s *PeerStore) RefreshServiceIDs(t ...time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	targetTime := time.Now()
	if len(t) > 0 {
		targetTime = t[0]
	}

	newMap := make(map[[16]byte]*Peer)
	for _, p := range s.all {
		id := GenerateServiceID(p.SharedSecret, targetTime)
		newMap[id] = p
		
		// Also allow the previous window to account for drift
		prevId := GenerateServiceID(p.SharedSecret, targetTime.Add(-WindowDuration))
		newMap[prevId] = p
	}
	s.peers = newMap
}

func (s *PeerStore) GetPeerByID(id [16]byte) (*Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.peers[id]
	return p, ok
}

func (s *PeerStore) GetPeerByPublicKey(pub [32]byte) (*Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.all {
		if p.PublicKey == pub {
			return p, true
		}
	}
	return nil, false
}
