package zdns

import (
	"testing"
	"time"
)

func TestPeerStore(t *testing.T) {
	secret := []byte("secret-shared-key-32-bytes-long!")
	peer := &Peer{
		Name:         "TestPeer",
		SharedSecret: secret,
	}

	store := NewPeerStore()
	store.AddPeer(peer)

	// Before refresh, store should be empty
	id := GenerateServiceID(secret, time.Now())
	if _, ok := store.GetPeerByID(id); ok {
		t.Errorf("Peer should not be found before RefreshServiceIDs")
	}

	store.RefreshServiceIDs()

	// After refresh, current ID should work
	if found, ok := store.GetPeerByID(id); !ok || found.Name != "TestPeer" {
		t.Errorf("Peer not found with current ServiceID")
	}

	// Test window drift (previous window)
	prevId := GenerateServiceID(secret, time.Now().Add(-WindowDuration))
	if found, ok := store.GetPeerByID(prevId); !ok || found.Name != "TestPeer" {
		t.Errorf("Peer not found with previous ServiceID (drift support)")
	}
}
