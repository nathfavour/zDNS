package main

import (
	"log"
	"time"

	"github.com/nathfavour/zdns/pkg/zdns"
)

func main() {
	// Simulated OOB Pairing data
	sharedSecret := []byte("this-is-a-32-byte-shared-secret!!") // Exactly 32 bytes for ChaCha20
	myPeer := &zdns.Peer{
		Name:         "MyPhone",
		SharedSecret: sharedSecret,
	}

	broadcaster, err := zdns.NewBroadcaster()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting Advertiser as %s...", myPeer.Name)
	for {
		log.Println("Broadcasting state...")
		err := broadcaster.Broadcast(myPeer, zdns.StateUnlocked, 85)
		if err != nil {
			log.Printf("Error broadcasting: %v", err)
		}
		time.Sleep(10 * time.Second)
	}
}
