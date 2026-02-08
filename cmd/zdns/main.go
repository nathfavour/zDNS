package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nathfavour/zdns/pkg/ipc"
	"github.com/nathfavour/zdns/pkg/sysinfo"
	"github.com/nathfavour/zdns/pkg/tui"
	"github.com/nathfavour/zdns/pkg/zdns"
)

// Global state for the daemon to track live peers
var (
	livePeers   = make(map[[32]byte]ipc.PeerStatus)
	livePeersMu sync.RWMutex
)

type Identity struct {
	Name    string   `json:"name"`
	Private [32]byte `json:"priv"`
	Public  [32]byte `json:"pub"`
}

func runDash() {
	storage, _ := zdns.NewStorage()
	socketPath := filepath.Join(storage.ConfigDir, "zdns", "zdns.sock")
	if err := tui.Run(socketPath); err != nil {
		log.Fatalf("TUI Error: %v", err)
	}
}

func runStatus() {
	storage, _ := zdns.NewStorage()
	socketPath := filepath.Join(storage.ConfigDir, "zdns", "zdns.sock")

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatalf("Is the daemon running? (Could not connect to %s: %v)", socketPath, err)
	}
	defer conn.Close()

	req := ipc.Request{Command: "list_peers"}
	json.NewEncoder(conn).Encode(req)

	var resp ipc.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		log.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != "" {
		log.Fatalf("Daemon error: %s", resp.Error)
	}

	if len(resp.Peers) == 0 {
		fmt.Println("No live peers discovered yet.")
		return
	}

	fmt.Printf("%-15s %-10s %-10s %-10s\n", "NAME", "BATTERY", "STATE", "ID")
	fmt.Println(strings.Repeat("-", 50))
	for _, p := range resp.Peers {
		fmt.Printf("%-15s %-10d %-10s %-10s\n", p.Name, p.Battery, p.State, p.PublicKey)
	}
}

func getIdentity(storage *zdns.Storage) (*Identity, error) {
	path := filepath.Join(storage.ConfigDir, "identity.json")
	if _, err := os.Stat(path); err == nil {
		data, _ := os.ReadFile(path)
		var id Identity
		json.Unmarshal(data, &id)
		return &id, nil
	}

	// Create new identity
	priv, pub, err := zdns.GenerateLongTermKey()
	if err != nil {
		return nil, err
	}
	id := &Identity{
		Name:    getHostname(),
		Private: priv,
		Public:  pub,
	}
	data, _ := json.Marshal(id)
	os.WriteFile(path, data, 0600)
	return id, nil
}

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
	case "daemon":
		runDaemon()
	case "status":
		runStatus()
	case "dash":
		runDash()
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
	fmt.Println("  zdns advertise                     Advertise current state")
	fmt.Println("  zdns daemon                        Run both listener and advertiser")
	fmt.Println("  zdns status                        Show status of discovered peers")
	fmt.Println("  zdns dash                          Show real-time TUI dashboard")
}

func runDaemon() {
	fmt.Println("Starting zDNS Daemon...")
	
	storage, _ := zdns.NewStorage()
	store := zdns.NewPeerStore()
	
	// Pre-load store for IPC
	peers, _ := storage.LoadPeers()
	for _, p := range peers {
		store.AddPeer(p)
	}

	ipcServer, err := ipc.NewServer(store)
	if err != nil {
		log.Fatalf("IPC init failed: %v", err)
	}

	ipcServer.GetStatus = func() []ipc.PeerStatus {
		livePeersMu.RLock()
		defer livePeersMu.RUnlock()
		status := make([]ipc.PeerStatus, 0, len(livePeers))
		for _, p := range livePeers {
			status = append(status, p)
		}
		return status
	}

	go ipcServer.Start()
	fmt.Printf("IPC Server active at: %s\n", ipcServer.SocketPath)

	// Run advertiser in background
	go runAdvertise()
	// Run listener in foreground
	runListen()
}

func runInvite() {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	storage, _ := zdns.NewStorage()
	id, err := getIdentity(storage)
	if err != nil {
		log.Fatalf("Failed to get identity: %v", err)
	}

	code, err := zdns.CreateInvite(id.Name, id.Public)
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
	days := fs.Int("days", 0, "Number of days trust should last (0 for infinite)")
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

	id, err := getIdentity(storage)
	if err != nil {
		log.Fatal(err)
	}

	// DH Handshake: My Private + Peer Public
	sharedSecret, err := zdns.DeriveSharedSecret(id.Private, invite.PublicKey)
	if err != nil {
		log.Fatalf("DH Handshake failed: %v", err)
	}

	var expiresAt int64
	if *days > 0 {
		expiresAt = time.Now().AddDate(0, 0, *days).Unix()
	}

	peers, _ := storage.LoadPeers()
	newPeer := &zdns.Peer{
		Name:         invite.Name,
		PublicKey:    invite.PublicKey,
		ExpiresAt:    expiresAt,
		SharedSecret: sharedSecret,
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
		// Update live status for IPC
		livePeersMu.Lock()
		livePeers[peer.PublicKey] = ipc.PeerStatus{
			Name:      peer.Name,
			Battery:   blob.BatteryLevel,
			State:     blob.DeviceState.String(),
			LastSeen:  time.Now().Unix(),
			PublicKey: fmt.Sprintf("%x", peer.PublicKey[:4]), // Short fingerprint
		}
		livePeersMu.Unlock()

		fmt.Printf("[%s] %s - Battery: %d%% - %s\n",
			time.Now().Format("15:04:05"), peer.Name, blob.BatteryLevel, blob.DeviceState)
	})
	if err != nil {
		log.Fatal(err)
	}
}

func runAdvertise() {
	fs := flag.NewFlagSet("advertise", flag.ExitOnError)
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

	info := sysinfo.NewInfoProvider()

	fmt.Printf("Advertising to %d peers...\n", len(peers))
	for {
		battery := info.GetBatteryLevel()
		state := info.GetDeviceState()

		for _, peer := range peers {
			err := broadcaster.Broadcast(peer, state, battery)
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