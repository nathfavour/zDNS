package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

				"github.com/nathfavour/zdns/pkg/commands"

				"github.com/nathfavour/zdns/pkg/dnsbridge"

				"github.com/nathfavour/zdns/pkg/ipc"

				"github.com/nathfavour/zdns/pkg/sysinfo"

			"github.com/nathfavour/zdns/pkg/triggers"

			"github.com/nathfavour/zdns/pkg/tui"

			"github.com/nathfavour/zdns/pkg/zdns"

		)

		

		// Global state for the daemon to track live peers

		var (

			livePeers     = make(map[[32]byte]ipc.PeerStatus)

			livePeersMu   sync.RWMutex

			triggerEngine *triggers.Engine

			cmdExecutor   *commands.Executor

			historyManager *zdns.HistoryManager

			masterPass    string

		)

	

	func getStorage() (*zdns.Storage, error) {

		pass := masterPass

		if pass == "" {

			pass = os.Getenv("ZDNS_PASSWORD")

		}

		return zdns.NewStorage(pass)

	}

	

	type Identity struct {
	Name    string   `json:"name"`
	Private [32]byte `json:"priv"`
	Public  [32]byte `json:"pub"`
}

func runExec() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zdns exec <peer_name> <command_name>")
		return
	}
	targetPeer := os.Args[2]
	targetCmd := os.Args[3]

	storage, err := getStorage()
	if err != nil {
		log.Fatal(err)
	}

	peers, _ := storage.LoadPeers()
	var peer *zdns.Peer
	for _, p := range peers {
		if p.Name == targetPeer {
			peer = p
			break
		}
	}

	if peer == nil {
		log.Fatalf("Peer '%s' not found in trusted list", targetPeer)
	}

	broadcaster, err := zdns.NewBroadcaster()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Sending command '%s' to %s...\n", targetCmd, targetPeer)
	// We send a one-off broadcast with the command
	// For simplicity, we use default state/battery values for this command-only packet
	err = broadcaster.Broadcast(peer, zdns.StateUnlocked, 100, "", targetCmd)
	if err != nil {
		log.Fatalf("Failed to send command: %v", err)
	}
	fmt.Println("Command sent.")
}

func runConnect() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zdns connect <peer_name> <service_tag>")
		return
	}
	targetPeer := os.Args[2]
	targetTag := os.Args[3]

	storage, _ := getStorage()
	socketPath := filepath.Join(storage.ConfigDir, "zdns", "zdns.sock")

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatalf("Daemon not running: %v", err)
	}
	defer conn.Close()

	json.NewEncoder(conn).Encode(ipc.Request{Command: "list_peers"})
	var resp ipc.Response
	json.NewDecoder(conn).Decode(&resp)

	var found *ipc.PeerStatus
	for _, p := range resp.Peers {
		if p.Name == targetPeer {
			found = &p
			break
		}
	}

	if found == nil {
		log.Fatalf("Peer '%s' not found or offline", targetPeer)
	}

	// Parse tags for the specific service
	// Tags format: "ssh:22,http:8080"
	port := ""
	tags := strings.Split(found.Tags, ",")
	for _, t := range tags {
		parts := strings.Split(t, ":")
		if parts[0] == targetTag {
			if len(parts) > 1 {
				port = parts[1]
			}
			break
		}
	}

	if port == "" && !strings.Contains(found.Tags, targetTag) {
		log.Fatalf("Service '%s' not advertised by %s (Tags: %s)", targetTag, targetPeer, found.Tags)
	}

	// Action!
	addr := found.IP
	if port != "" {
		addr = net.JoinHostPort(addr, port)
	}

	fmt.Printf("Connecting to %s on %s via %s...\n", targetPeer, addr, targetTag)

	var cmd *exec.Cmd
	switch targetTag {
	case "ssh":
		user := os.Getenv("USER")
		pArg := "-p"
		if port == "" {
			port = "22"
		}
		cmd = exec.Command("ssh", fmt.Sprintf("%s@%s", user, found.IP), pArg, port)
	case "http", "https":
		cmd = exec.Command("xdg-open", fmt.Sprintf("%s://%s", targetTag, addr))
	default:
		fmt.Printf("No default handler for '%s'. Address: %s\n", targetTag, addr)
		return
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}

func runDash() {
	storage, _ := getStorage()
	socketPath := filepath.Join(storage.ConfigDir, "zdns", "zdns.sock")
	if err := tui.Run(socketPath); err != nil {
		log.Fatalf("TUI Error: %v", err)
	}
}

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

	// Start a local listener for the file transfer
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Fatalf("Failed to start file server: %v", err)
	}
	defer l.Close()

	_, port, _ := net.SplitHostPort(l.Addr().String())
	fileName := filepath.Base(filePath)

	broadcaster, _ := zdns.NewBroadcaster()
	
	// Signal the peer: drop:<port>:<filename>
	dropCmd := fmt.Sprintf("drop:%s:%s", port, fileName)
	fmt.Printf("Signaling %s to receive '%s' on port %s...\n", targetName, fileName, port)
	
	err = broadcaster.Broadcast(targetPeer, zdns.StateUnlocked, 100, "", dropCmd)
	if err != nil {
		log.Fatalf("Failed to signal peer: %v", err)
	}

	// Serve the file
	if err := zdns.SendFile(filePath, targetPeer, l); err != nil {
		log.Fatalf("File transfer failed: %v", err)
	}
}

func runStatus() {
	storage, _ := getStorage()
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

	fmt.Printf("%-15s %-10s %-10s %-20s %-10s %-10s\n", "NAME", "BATTERY", "STATE", "SERVICES", "LATENCY", "ID")
	fmt.Println(strings.Repeat("-", 85))
	for _, p := range resp.Peers {
		latency := "???"
		if p.Latency > 0 {
			latency = fmt.Sprintf("%dms", p.Latency)
		}
		fmt.Printf("%-15s %-10d %-10s %-20s %-10s %-10s\n", p.Name, p.Battery, p.State, p.Tags, latency, p.PublicKey)
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

	// Simple global flag check
	for i, arg := range os.Args {
		if arg == "--pass" && i+1 < len(os.Args) {
			masterPass = os.Args[i+1]
			// Remove the flag so it doesn't break subcommands
			os.Args = append(os.Args[:i], os.Args[i+2:]...)
			break
		}
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
	case "connect":
		runConnect()
	case "peers":
		runPeers()
	case "exec":
		runExec()
	case "history":
		runHistory()
	case "proxy":
		runProxy()
	case "send":
		runSend()
	case "relay":
		runRelayServer()
	case "identity":
		runIdentity()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("zDNS - Zero-Trust Discovery Service")
	fmt.Println("\nGlobal Flags:")
	fmt.Println("  --pass PASSWORD                    Master password for the vault (or use ZDNS_PASSWORD env)")
	fmt.Println("\nUsage:")
	fmt.Println("  zdns invite [--name NAME]          Generate a pairing invite")
	fmt.Println("  zdns join --invite INVITE_CODE     Join a peer using an invite code")
	fmt.Println("  zdns listen                        Listen for trusted peers")
	fmt.Println("  zdns advertise                     Advertise current state")
	fmt.Println("  zdns daemon [--relay URL]          Run both listener and advertiser")
	fmt.Println("  zdns status                        Show status of discovered peers")
	fmt.Println("  zdns dash                          Show real-time TUI dashboard")
	fmt.Println("  zdns connect <peer> <service>      Connect to a discovered service")
	fmt.Println("  zdns proxy <peer> <lport>:<rport>  Proxy a local port to a remote service")
	fmt.Println("  zdns send <peer> <file_path>       Send an encrypted file to a peer")
	fmt.Println("  zdns peers [list|rm|rename|sync]   Manage trusted peers")
	fmt.Println("  zdns exec <peer> <command>         Execute a remote safe command")
	fmt.Println("  zdns history                       Show encrypted event history")
	fmt.Println("  zdns identity [export|import]      Backup or restore your identity")
	fmt.Println("  zdns relay                         Start a standalone signaling server")
}

func runIdentity() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: zdns identity [export|import <hex>]")
		return
	}

	storage, _ := getStorage()
	path := filepath.Join(storage.ConfigDir, "identity.json")

	switch os.Args[2] {
	case "export":
		id, err := getIdentity(storage)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("--- zDNS Identity Backup ---")
		fmt.Println("Your Paper-Key (Private Key):")
		fmt.Printf("\n%s\n\n", zdns.ExportKey(id.Private))
		fmt.Println("KEEP THIS SECRET. Anyone with this key can impersonate your device.")
		fmt.Println("-----------------------------")
	case "import":
		if len(os.Args) < 4 {
			log.Fatal("Usage: zdns identity import <hex_key>")
		}
		priv, err := zdns.ImportKey(os.Args[3])
		if err != nil {
			log.Fatalf("Invalid key: %v", err)
		}
		pub := zdns.ReconstitutePublic(priv)
		id := &Identity{
			Name:    getHostname(),
			Private: priv,
			Public:  pub,
		}
		data, _ := json.Marshal(id)
		os.WriteFile(path, data, 0600)
		fmt.Println("Successfully imported identity. Restart the daemon to apply.")
	}
}

func runRelayServer() {
	fs := flag.NewFlagSet("relay", flag.ExitOnError)
	port := fs.String("port", "8080", "Port to listen on")
	fs.Parse(os.Args[2:])

	// In-memory store for check-ins (IDHash -> RelayCheckIn)
	store := make(map[string]zdns.RelayCheckIn)
	var mu sync.RWMutex

	http.HandleFunc("/checkin", func(w http.ResponseWriter, r *http.Request) {
		var c zdns.RelayCheckIn
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mu.Lock()
		store[fmt.Sprintf("%x", c.IDHash)] = c
		mu.Unlock()
		w.WriteHeader(200)
	})

	http.HandleFunc("/lookup/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/lookup/")
		mu.RLock()
		c, ok := store[id]
		mu.RUnlock()
		if !ok {
			http.Error(w, "Not Found", 404)
			return
		}
		json.NewEncoder(w).Encode(c)
	})

	fmt.Printf("zDNS Signaling Relay starting on :%s...\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

func runProxy() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: zdns proxy <peer_name> <local_port>:<remote_tag_or_port>")
		return
	}
	targetPeer := os.Args[2]
	mapping := os.Args[3]
	parts := strings.Split(mapping, ":")
	if len(parts) != 2 {
		log.Fatal("Invalid mapping format. Use local_port:remote_target")
	}
	localPort, remoteTarget := parts[0], parts[1]

	storage, _ := getStorage()
	socketPath := filepath.Join(storage.ConfigDir, "zdns", "zdns.sock")

	// Start local listener
	l, err := net.Listen("tcp", "127.0.0.1:"+localPort)
	if err != nil {
		log.Fatalf("Failed to listen on localhost:%s: %v", localPort, err)
	}
	fmt.Printf("Proxying localhost:%s -> %s (%s)...\n", localPort, targetPeer, remoteTarget)

	for {
		clientConn, err := l.Accept()
		if err != nil {
			continue
		}

		// On every new connection, re-resolve the peer IP/Port from the daemon
		go func() {
			defer clientConn.Close()
			
			// Dial daemon
			dConn, err := net.Dial("unix", socketPath)
			if err != nil {
				return
			}
			json.NewEncoder(dConn).Encode(ipc.Request{Command: "list_peers"})
			var resp ipc.Response
			json.NewDecoder(dConn).Decode(&resp)
			dConn.Close()

			var peer *ipc.PeerStatus
			for _, p := range resp.Peers {
				if p.Name == targetPeer {
					peer = &p
					break
				}
			}
			if peer == nil {
				return
			}

			// Resolve remote port
			rPort := remoteTarget
			tags := strings.Split(peer.Tags, ",")
			for _, t := range tags {
				tp := strings.Split(t, ":")
				if tp[0] == remoteTarget && len(tp) > 1 {
					rPort = tp[1]
					break
				}
			}

			// Dial remote peer
			remoteAddr := net.JoinHostPort(peer.IP, rPort)
			remoteConn, err := net.DialTimeout("tcp", remoteAddr, 5*time.Second)
			if err != nil {
				return
			}
			defer remoteConn.Close()

			// Bridge them
			errChan := make(chan error, 2)
			go func() {
				_, err := io.Copy(remoteConn, clientConn)
				errChan <- err
			}()
			go func() {
				_, err := io.Copy(clientConn, remoteConn)
				errChan <- err
			}()

			<-errChan
		}()
	}
}

func runHistory() {
	storage, err := getStorage()
	if err != nil {
		log.Fatal(err)
	}

	hm, err := zdns.NewHistoryManager(storage.ConfigDir, storage.GetVault())
	if err != nil {
		log.Fatalf("Failed to open history: %v", err)
	}

	events := hm.GetEvents()
	if len(events) == 0 {
		fmt.Println("No history recorded yet.")
		return
	}

	fmt.Printf("%-20s %-15s %-10s %-20s\n", "TIME", "PEER", "TYPE", "DATA")
	fmt.Println(strings.Repeat("-", 70))
	for _, e := range events {
		t := time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")
		fmt.Printf("%-20s %-15s %-10s %-20s\n", t, e.PeerName, e.Type, e.Data)
	}
}

func runPeers() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: zdns peers [list|rm|rename]")
		return
	}

	storage, _ := getStorage()
	peers, _ := storage.LoadPeers()

	switch os.Args[2] {
	case "list":
		fmt.Printf("%-20s %-20s %-20s\n", "NAME", "ID (Fingerprint)", "EXPIRES")
		fmt.Println(strings.Repeat("-", 60))
		for _, p := range peers {
			expires := "Never"
			if p.ExpiresAt > 0 {
				expires = time.Unix(p.ExpiresAt, 0).Format("2006-01-02")
			}
			fmt.Printf("%-20s %-20x %-20s\n", p.Name, p.PublicKey[:4], expires)
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
				fmt.Printf("Removed peer: %s\n", target)
				continue
			}
			newPeers = append(newPeers, p)
		}
		storage.SavePeers(newPeers)
	case "sync":
		if len(os.Args) < 4 {
			log.Fatal("Usage: zdns peers sync <peer_name>")
		}
		targetName := os.Args[3]
		
		// 1. Start a temporary TCP listener to receive the peer list
		l, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			log.Fatal(err)
		}
		defer l.Close()
		_, port, _ := net.SplitHostPort(l.Addr().String())

		// 2. Resolve target peer
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

		// 3. Send sync request: sync_req:port
		broadcaster, _ := zdns.NewBroadcaster()
		fmt.Printf("Requesting peer sync from %s...\n", targetName)
		broadcaster.Broadcast(targetPeer, zdns.StateUnlocked, 100, "", "sync_req:"+port)

		// 4. Accept connection and receive JSON
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		var syncedPeers []zdns.Peer
		if err := json.NewDecoder(conn).Decode(&syncedPeers); err != nil {
			log.Fatalf("Failed to decode synced peers: %v", err)
		}

		// 5. Merge new peers (Metadata only, they will need pairing for secrets)
		added := 0
		existingMap := make(map[[32]byte]bool)
		for _, p := range peers {
			existingMap[p.PublicKey] = true
		}

		for _, sp := range syncedPeers {
			if !existingMap[sp.PublicKey] {
				peers = append(peers, &sp)
				added++
				fmt.Printf("Discovered new peer: %s\n", sp.Name)
			}
		}
		
		if added > 0 {
			storage.SavePeers(peers)
			fmt.Printf("Sync complete. Added %d new potential peers.\n", added)
		} else {
			fmt.Println("Sync complete. No new peers found.")
		}
	}
}

func runDaemon() {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	tags := fs.String("tags", "", "Comma-separated service tags")
	dnsAddr := fs.String("dns", "", "Enable DNS bridge on address (e.g. 127.0.0.1:5353)")
	relayURL := fs.String("relay", "", "Signaling relay URL (e.g. http://my-relay.com:8080)")
	fs.Parse(os.Args[2:])

	fmt.Println("Starting zDNS Daemon...")
	
	storage, _ := getStorage()
	triggerEngine, _ = triggers.NewEngine(storage.ConfigDir)
	cmdExecutor, _ = commands.NewExecutor(storage.ConfigDir)
	historyManager, _ = zdns.NewHistoryManager(storage.ConfigDir, storage.GetVault())

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

	statusFunc := func() []ipc.PeerStatus {
		livePeersMu.RLock()
		defer livePeersMu.RUnlock()
		status := make([]ipc.PeerStatus, 0, len(livePeers))
		for _, p := range livePeers {
			status = append(status, p)
		}
		return status
	}

	ipcServer.GetStatus = statusFunc
	go ipcServer.Start()
	fmt.Printf("IPC Server active at: %s\n", ipcServer.SocketPath)

	// Start DNS Bridge if requested
	if *dnsAddr != "" {
		dnsServer := dnsbridge.NewServer(*dnsAddr)
		dnsServer.GetStatus = statusFunc
		go dnsServer.Start()
	}

	// Background Task: Relay Integration
	if *relayURL != "" {
		id, _ := getIdentity(storage)
		info := sysinfo.NewInfoProvider()
		
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			for range ticker.C {
				// 1. Check-in myself
				for _, peer := range peers {
					blob := &zdns.StateBlob{
						Type:         zdns.TypeStatus,
						DeviceID:     id.Public,
						DeviceState:  info.GetDeviceState(),
						BatteryLevel: info.GetBatteryLevel(),
						Timestamp:    time.Now().Unix(),
						Tags:         *tags,
					}
					zdns.PostToRelay(*relayURL, id.Public, peer.SharedSecret, blob)
				}

				// 2. Query for peers
				for _, peer := range peers {
					blob, err := zdns.QueryRelay(*relayURL, peer.PublicKey, peer.SharedSecret)
					if err == nil {
						// Update livePeers with relay data
						livePeersMu.Lock()
						livePeers[peer.PublicKey] = ipc.PeerStatus{
							Name:      peer.Name,
							IP:        net.IP(blob.IP[:]).String() + " (Relay)",
							Battery:   blob.BatteryLevel,
							State:     blob.DeviceState.String(),
							LastSeen:  time.Now().Unix(),
							PublicKey: fmt.Sprintf("%x", peer.PublicKey[:4]),
							Tags:      blob.Tags,
						}
						livePeersMu.Unlock()
					}
				}
			}
		}()
	}

	// Background Task: Periodic PING to live peers for latency
	go func() {
		broadcaster, _ := zdns.NewBroadcaster()
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			livePeersMu.RLock()
			for pubKeyArr, p := range livePeers {
				peer, ok := store.GetPeerByPublicKey(pubKeyArr)
				if ok {
					blob := &zdns.StateBlob{
						Type:      zdns.TypePing,
						Timestamp: time.Now().Unix(),
					}
					broadcaster.SendTo(peer, p.IP+":5354", blob)
				}
			}
			livePeersMu.RUnlock()
		}
	}()

	// Background Task: Cleanup stale peers (not seen for > 15 mins)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			livePeersMu.Lock()
			now := time.Now().Unix()
			for pub, p := range livePeers {
				if now-p.LastSeen > 900 { // 15 minutes
					delete(livePeers, pub)
				}
			}
			livePeersMu.Unlock()
		}
	}()

	// Run advertiser in background
	go runAdvertiseWithTags(*tags)
	// Run listener in foreground
	runListen()
}

func runAdvertiseWithTags(tags string) {
	storage, _ := getStorage()
	peers, _ := storage.LoadPeers()
	if len(peers) == 0 {
		return
	}

	broadcaster, _ := zdns.NewBroadcaster()
	info := sysinfo.NewInfoProvider()

	for {
		battery := info.GetBatteryLevel()
		state := info.GetDeviceState()
		
		// Pulse: Health Check
		liveTags := sysinfo.ProbeTags(tags)

		for _, peer := range peers {
			broadcaster.Broadcast(peer, state, battery, liveTags, "")
		}

		// Randomized sleep (7-14s)
		var b [1]byte
		rand.Read(b[:])
		jitter := int(b[0] % 8)
		time.Sleep(time.Duration(7+jitter) * time.Second)

		// 20% chance to send Chaff
		if b[0]%5 == 0 {
			broadcaster.BroadcastChaff()
		}
	}
}

func runInvite() {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	storage, _ := getStorage()
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

	storage, err := getStorage()
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
	storage, err := getStorage()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize triggers
	triggerEngine, _ = triggers.NewEngine(storage.ConfigDir)
	cmdExecutor, _ = commands.NewExecutor(storage.ConfigDir)
	historyManager, _ = zdns.NewHistoryManager(storage.ConfigDir, storage.GetVault())

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

	broadcaster, _ := zdns.NewBroadcaster()

	fmt.Println("zDNS Listener Active. Waiting for trusted peers...")
	err = listener.Listen(func(peer *zdns.Peer, blob *zdns.StateBlob) {
		peerAddr := net.IP(blob.IP[:]).String() + ":5354"
		
		if blob.Type == zdns.TypePing {
			// Respond with PONG
			resp := *blob
			resp.Type = zdns.TypePong
			broadcaster.SendTo(peer, peerAddr, &resp)
			return
		}

		if blob.Type == zdns.TypePong {
			// Calculate RTT
			// Since our TS is only seconds for now, let's just mark it seen.
			// In a real impl, we'd use a higher res TS for PING/PONG
			livePeersMu.Lock()
			if p, ok := livePeers[peer.PublicKey]; ok {
				p.Latency = time.Now().UnixMilli() - (blob.Timestamp * 1000)
				livePeers[peer.PublicKey] = p
			}
			livePeersMu.Unlock()
			return
		}

		newState := blob.DeviceState.String()

		livePeersMu.Lock()
		oldStatus, exists := livePeers[peer.PublicKey]
		
		livePeers[peer.PublicKey] = ipc.PeerStatus{
			Name:      peer.Name,
			IP:        net.IP(blob.IP[:]).String(),
			Battery:   blob.BatteryLevel,
			State:     newState,
			LastSeen:  time.Now().Unix(),
			PublicKey: fmt.Sprintf("%x", peer.PublicKey[:4]),
			Tags:      blob.Tags,
			Latency:   oldStatus.Latency,
		}
		livePeersMu.Unlock()

		if !exists {
			historyManager.Log(peer.Name, "JOIN", "Device appeared on network")
		}

		// Trigger Check: Only if state actually changed
		if exists && oldStatus.State != newState {
			historyManager.Log(peer.Name, "STATE", fmt.Sprintf("%s -> %s", oldStatus.State, newState))
			env := map[string]string{
				"ZDNS_PEER_NAME": peer.Name,
				"ZDNS_PEER_BAT":  fmt.Sprintf("%d", blob.BatteryLevel),
				"ZDNS_PEER_TAGS": blob.Tags,
			}
			triggerEngine.CheckAndFire(peer.Name, triggers.EventStateChange, newState, env)
		}

		// Remote Command Execution
		if blob.Command != "" {
			if strings.HasPrefix(blob.Command, "drop:") {
				// Handle file drop: drop:port:filename
				parts := strings.Split(blob.Command, ":")
				if len(parts) >= 3 {
					port, fileName := parts[1], parts[2]
					senderAddr := net.JoinHostPort(net.IP(blob.IP[:]).String(), port)
					dropDir := filepath.Join(storage.ConfigDir, "drops")
					os.MkdirAll(dropDir, 0700)
					
					historyManager.Log(peer.Name, "DROP", "Receiving file: "+fileName)
					go func() {
						if err := zdns.ReceiveFile(senderAddr, peer, fileName, dropDir); err != nil {
							fmt.Printf("\n[DROP ERROR] Failed to receive %s: %v\n", fileName, err)
						} else {
							fmt.Printf("\n[DROP SUCCESS] Received %s in %s\n", fileName, dropDir)
						}
					}()
				}
			} else if strings.HasPrefix(blob.Command, "sync_req:") {
				// Handle peer metadata sync request
				parts := strings.Split(blob.Command, ":")
				if len(parts) >= 2 {
					port := parts[1]
					targetAddr := net.JoinHostPort(net.IP(blob.IP[:]).String(), port)
					historyManager.Log(peer.Name, "SYNC", "Sending peer list")
					
					go func() {
						conn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
						if err != nil {
							return
						}
						defer conn.Close()
						
						// Export and send metadata
						meta := store.ExportMetadata()
						json.NewEncoder(conn).Encode(meta)
					}()
				}
			} else {
				historyManager.Log(peer.Name, "EXEC", blob.Command)
				go cmdExecutor.Execute(blob.Command)
			}
		}

		fmt.Printf("[%s] %s - Battery: %d%% - %s\n",
			time.Now().Format("15:04:05"), peer.Name, blob.BatteryLevel, blob.DeviceState)
	})
	if err != nil {
		log.Fatal(err)
	}
}

func runAdvertise() {
	fs := flag.NewFlagSet("advertise", flag.ExitOnError)
	tags := fs.String("tags", "", "Comma-separated service tags")
	fs.Parse(os.Args[2:])

	storage, err := getStorage()
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
		
		// Pulse: Health Check
		liveTags := sysinfo.ProbeTags(*tags)

		for _, peer := range peers {
			err := broadcaster.Broadcast(peer, state, battery, liveTags, "")
			if err != nil {
				log.Printf("Broadcast error: %v", err)
			}
		}

		// Randomized sleep (7-14s) to break timing analysis
		var b [1]byte
		rand.Read(b[:])
		jitter := int(b[0] % 8)
		time.Sleep(time.Duration(7+jitter) * time.Second)

		// 20% chance to send Chaff (noise)
		if b[0]%5 == 0 {
			broadcaster.BroadcastChaff()
		}
	}
}

func getHostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "UnknownDevice"
	}
	return h
}