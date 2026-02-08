package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathfavour/zdns/pkg/commands"
	"github.com/nathfavour/zdns/pkg/dnsbridge"
	"github.com/nathfavour/zdns/pkg/ipc"
	"github.com/nathfavour/zdns/pkg/sysinfo"
	"github.com/nathfavour/zdns/pkg/triggers"
	"github.com/nathfavour/zdns/pkg/zdns"
)

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
	fmt.Printf("IPC Server active at: %s
", ipcServer.SocketPath)

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

				for _, peer := range peers {
					blob, err := zdns.QueryRelay(*relayURL, peer.PublicKey, peer.SharedSecret)
					if err == nil {
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
		
		liveTags := sysinfo.ProbeTags(tags)

		for _, peer := range peers {
			broadcaster.Broadcast(peer, state, battery, liveTags, "")
		}

		var b [1]byte
		rand.Read(b[:])
		jitter := int(b[0] % 8)
		time.Sleep(time.Duration(7+jitter) * time.Second)

		if b[0]%5 == 0 {
			broadcaster.BroadcastChaff()
		}
	}
}

func runListen() {
	storage, err := getStorage()
	if err != nil {
		log.Fatal(err)
	}

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
			resp := *blob
			resp.Type = zdns.TypePong
			broadcaster.SendTo(peer, peerAddr, &resp)
			return
		}

		if blob.Type == zdns.TypePong {
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

		if exists && oldStatus.State != newState {
			historyManager.Log(peer.Name, "STATE", fmt.Sprintf("%s -> %s", oldStatus.State, newState))
			env := map[string]string{
				"ZDNS_PEER_NAME": peer.Name,
				"ZDNS_PEER_BAT":  fmt.Sprintf("%d", blob.BatteryLevel),
				"ZDNS_PEER_TAGS": blob.Tags,
			}
			triggerEngine.CheckAndFire(peer.Name, triggers.EventStateChange, newState, env)
		}

		if blob.Command != "" {
			if strings.HasPrefix(blob.Command, "drop:") {
				parts := strings.Split(blob.Command, ":")
				if len(parts) >= 3 {
					port, fileName := parts[1], parts[2]
					senderAddr := net.JoinHostPort(net.IP(blob.IP[:]).String(), port)
					dropDir := filepath.Join(storage.ConfigDir, "drops")
					os.MkdirAll(dropDir, 0700)
					
					historyManager.Log(peer.Name, "DROP", "Receiving file: "+fileName)
					go func() {
						if err := zdns.ReceiveFile(senderAddr, peer, fileName, dropDir); err != nil {
							fmt.Printf("
[DROP ERROR] Failed to receive %s: %v
", fileName, err)
						} else {
							fmt.Printf("
[DROP SUCCESS] Received %s in %s
", fileName, dropDir)
						}
					}()
				}
			} else if strings.HasPrefix(blob.Command, "sync_req:") {
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
						meta := store.ExportMetadata()
						json.NewEncoder(conn).Encode(meta)
					}()
				}
			} else {
				historyManager.Log(peer.Name, "EXEC", blob.Command)
				go cmdExecutor.Execute(blob.Command)
			}
		}

		fmt.Printf("[%s] %s - Battery: %d%% - %s
",
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

	fmt.Printf("Advertising to %d peers...
", len(peers))
	for {
		battery := info.GetBatteryLevel()
		state := info.GetDeviceState()
		
		liveTags := sysinfo.ProbeTags(*tags)

		for _, peer := range peers {
			err := broadcaster.Broadcast(peer, state, battery, liveTags, "")
			if err != nil {
				log.Printf("Broadcast error: %v", err)
			}
		}

		var b [1]byte
		rand.Read(b[:])
		jitter := int(b[0] % 8)
		time.Sleep(time.Duration(7+jitter) * time.Second)

		if b[0]%5 == 0 {
			broadcaster.BroadcastChaff()
		}
	}
}
