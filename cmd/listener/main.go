package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nathfavour/zdns/pkg/zdns"
)

func main() {
	storage, err := zdns.NewStorage()
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	store := zdns.NewPeerStore()
	
	// Load existing peers
	peers, err := storage.LoadPeers()
	if err != nil {
		log.Printf("Warning: Failed to load peers: %v", err)
	}
	
	if len(peers) == 0 {
		log.Println("No peers found. Adding default test peer...")
		sharedSecret := []byte("this-is-a-32-byte-shared-secret!!")
		trustedPeer := &zdns.Peer{
			Name:         "MyPhone",
			SharedSecret: sharedSecret,
		}
		store.AddPeer(trustedPeer)
		// Save for next time
		storage.SavePeers([]*zdns.Peer{trustedPeer})
	} else {
		for _, p := range peers {
			store.AddPeer(p)
		}
	}

	store.RefreshServiceIDs()

	// Periodically refresh ServiceIDs to handle rotation
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			store.RefreshServiceIDs()
		}
	}()

	listener, err := zdns.NewListener(store)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Starting zDNS Listener (Zero-Trust Mode)...")
	err = listener.Listen(func(peer *zdns.Peer, blob *zdns.StateBlob) {
		fmt.Printf("Found [%s] - Battery: %d%% - %s (TS: %d)\n",
			peer.Name, blob.BatteryLevel, blob.DeviceState, blob.Timestamp)
	})
	if err != nil {
		log.Fatal(err)
	}
}
