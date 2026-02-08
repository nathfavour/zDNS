package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/nathfavour/zdns/pkg/zdns"
)

func runSend() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zdns send <peer_name> <file_path>")
		return
	}
	targetName := os.Args[2]
	filePath := os.Args[3]

	storage, _ := getStorage()
	peers, _ := storage.LoadPeers()
	var targetPeer *zdns.Peer
	for _, p := range peers {
		if p.Name == targetName {
			targetPeer = p
			break
		}
	}

	if targetPeer == nil {
		log.Fatalf("Peer '%s' not found", targetName)
	}

	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Fatalf("Failed to start file server: %v", err)
	}
	defer l.Close()

	_, port, _ := net.SplitHostPort(l.Addr().String())
	fileName := filepath.Base(filePath)

	broadcaster, _ := zdns.NewBroadcaster()
	
	dropCmd := fmt.Sprintf("drop:%s:%s", port, fileName)
	fmt.Printf("Signaling %s to receive '%s' on port %s...\n", targetName, fileName, port)
	
	err = broadcaster.Broadcast(targetPeer, zdns.StateUnlocked, 100, "", dropCmd)
	if err != nil {
		log.Fatalf("Failed to signal peer: %v", err)
	}

	if err := zdns.SendFile(filePath, targetPeer, l); err != nil {
		log.Fatalf("File transfer failed: %v", err)
	}
}
