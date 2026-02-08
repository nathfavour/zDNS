package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nathfavour/zdns/pkg/zdns"
)

func main() {
	// Simulated OOB Pairing data (must match advertiser)
	sharedSecret := []byte("this-is-a-32-byte-shared-secret!!")
	trustedPeer := &zdns.Peer{
		Name:         "MyPhone",
		SharedSecret: sharedSecret,
	}

	store := zdns.NewPeerStore()
	store.AddPeer(trustedPeer)
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
