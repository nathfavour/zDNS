package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/nathfavour/zdns/pkg/zdns"
)

func runPeers() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: zdns peers [list|rm|rename|sync]")
		return
	}

	storage, _ := getStorage()
	peers, _ := storage.LoadPeers()

	switch os.Args[2] {
	case "list":
		fmt.Printf("%-20s %-20s %-20s
", "NAME", "ID (Fingerprint)", "EXPIRES")
		fmt.Println(strings.Repeat("-", 60))
		for _, p := range peers {
			expires := "Never"
			if p.ExpiresAt > 0 {
				expires = time.Unix(p.ExpiresAt, 0).Format("2006-01-02")
			}
			fmt.Printf("%-20s %-20x %-20s
", p.Name, p.PublicKey[:4], expires)
		}
	case "rm":
		if len(os.Args) < 4 {
			log.Fatal("Usage: zdns peers rm <name>")
		}
		target := os.Args[3]
		newPeers := []*zdns.Peer{}
		for _, p := range peers {
			if p.Name == target {
				storage.Secrets.RemoveSecret(p.PublicKey)
				fmt.Printf("Removed peer: %s
", target)
				continue
			}
			newPeers = append(newPeers, p)
		}
		storage.SavePeers(newPeers)
	case "rename":
		if len(os.Args) < 5 {
			log.Fatal("Usage: zdns peers rename <old> <new>")
		}
		oldName, newName := os.Args[3], os.Args[4]
		for _, p := range peers {
			if p.Name == oldName {
				p.Name = newName
				fmt.Printf("Renamed %s to %s
", oldName, newName)
			}
		}
		storage.SavePeers(peers)
	case "sync":
		if len(os.Args) < 4 {
			log.Fatal("Usage: zdns peers sync <peer_name>")
		}
		targetName := os.Args[3]
		
		l, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			log.Fatal(err)
		}
		defer l.Close()
		_, port, _ := net.SplitHostPort(l.Addr().String())

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

		broadcaster, _ := zdns.NewBroadcaster()
		fmt.Printf("Requesting peer sync from %s...
", targetName)
		broadcaster.Broadcast(targetPeer, zdns.StateUnlocked, 100, "", "sync_req:"+port)

		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		var syncedPeers []zdns.Peer
		if err := json.NewDecoder(conn).Decode(&syncedPeers); err != nil {
			log.Fatalf("Failed to decode synced peers: %v", err)
		}

		added := 0
		existingMap := make(map[[32]byte]bool)
		for _, p := range peers {
			existingMap[p.PublicKey] = true
		}

		for _, sp := range syncedPeers {
			if !existingMap[sp.PublicKey] {
				peers = append(peers, &sp)
				added++
				fmt.Printf("Discovered new peer: %s
", sp.Name)
			}
		}
		
		if added > 0 {
			storage.SavePeers(peers)
			fmt.Printf("Sync complete. Added %d new potential peers.
", added)
		} else {
			fmt.Println("Sync complete. No new peers found.")
		}
	}
}
