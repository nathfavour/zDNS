package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathfavour/zdns/pkg/ipc"
	"github.com/nathfavour/zdns/pkg/zdns"
)

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

		go func() {
			defer clientConn.Close()
			
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

			rPort := remoteTarget
			tags := strings.Split(peer.Tags, ",")
			for _, t := range tags {
				tp := strings.Split(t, ":")
				if tp[0] == remoteTarget && len(tp) > 1 {
					rPort = tp[1]
					break
				}
			}

			remoteAddr := net.JoinHostPort(peer.IP, rPort)
			remoteConn, err := net.DialTimeout("tcp", remoteAddr, 5*time.Second)
			if err != nil {
				return
			}
			defer remoteConn.Close()

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
