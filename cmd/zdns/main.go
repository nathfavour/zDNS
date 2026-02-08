package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nathfavour/zdns/pkg/zdns"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "invite":
		runInvite()
	case "join":
		runJoin()
	case "listen":
		runListen()
	case "advertise":
		runAdvertise()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("zDNS - Zero-Trust Discovery Service")
	fmt.Println("\nUsage:")
	fmt.Println("  zdns invite [--name NAME]          Generate a pairing invite")
	fmt.Println("  zdns join --invite INVITE_CODE     Join a peer using an invite code")
	fmt.Println("  zdns listen                        Listen for trusted peers")
	fmt.Println("  zdns advertise [--name NAME]       Advertise current state")
}

func runInvite() {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)
	name := fs.String("name", getHostname(), "Human-readable name for this device")
	fs.Parse(os.Args[2:])

	_, code, err := zdns.CreateInvite(*name)
	if err != nil {
		log.Fatalf("Failed to create invite: %v", err)
	}

	fmt.Println("--- zDNS Pairing Invite ---")
	fmt.Println("Share this code with your peer:")
	fmt.Println("\n" + code + "\n")
	fmt.Println("---------------------------")
}

func runJoin() {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	code := fs.String("invite", "", "The pairing code from your peer")
	fs.Parse(os.Args[2:])

	if *code == "" {
		log.Fatal("Error: --invite code is required")
	}

	invite, err := zdns.ParseInvite(*code)
	if err != nil {
		log.Fatalf("Failed to parse invite: %v", err)
	}

	storage, err := zdns.NewStorage()
	if err != nil {
		log.Fatal(err)
	}

	peers, _ := storage.LoadPeers()
	newPeer := &zdns.Peer{
		Name:         invite.Name,
		PublicKey:    invite.PublicKey,
		SharedSecret: invite.SharedSecret,
	}
	peers = append(peers, newPeer)

	if err := storage.SavePeers(peers); err != nil {
		log.Fatalf("Failed to save peer: %v", err)
	}

	fmt.Printf("Successfully joined peer: %s\n", invite.Name)
}

func runListen() {
	storage, err := zdns.NewStorage()
	if err != nil {
		log.Fatal(err)
	}

	store := zdns.NewPeerStore()
	peers, err := storage.LoadPeers()
	if err != nil {
		log.Fatalf("Failed to load peers: %v", err)
	}

	if len(peers) == 0 {
		fmt.Println("No trusted peers found. Use 'zdns join' first.")
		return
	}

	for _, p := range peers {
		store.AddPeer(p)
	}
	store.RefreshServiceIDs()

	// Periodic refresh for rolling IDs
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

	fmt.Println("zDNS Listener Active. Waiting for trusted peers...")
	err = listener.Listen(func(peer *zdns.Peer, blob *zdns.StateBlob) {
		fmt.Printf("[%s] %s - Battery: %d%% - %s\n",
			time.Now().Format("15:04:05"), peer.Name, blob.BatteryLevel, blob.DeviceState)
	})
	if err != nil {
		log.Fatal(err)
	}
}

func runAdvertise() {
	fs := flag.NewFlagSet("advertise", flag.ExitOnError)
	name := fs.String("name", getHostname(), "Name to broadcast")
	fs.Parse(os.Args[2:])

	storage, err := zdns.NewStorage()
	if err != nil {
		log.Fatal(err)
	}

	peers, _ := storage.LoadPeers()
	if len(peers) == 0 {
		fmt.Println("No peers found to advertise to. Use 'zdns join' first.")
		return
	}

	broadcaster, err := zdns.NewBroadcaster()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Advertising to %d peers...\n", len(peers))
	for {
		for _, peer := range peers {
			err := broadcaster.Broadcast(peer, zdns.StateUnlocked, 85)
			if err != nil {
				log.Printf("Broadcast error: %v", err)
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func getHostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "UnknownDevice"
	}
	return h
}