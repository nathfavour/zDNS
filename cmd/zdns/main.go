package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/nathfavour/zdns/pkg/commands"
	"github.com/nathfavour/zdns/pkg/ipc"
	"github.com/nathfavour/zdns/pkg/triggers"
	"github.com/nathfavour/zdns/pkg/zdns"
)

// Global state for the daemon to track live peers
var (
	livePeers      = make(map[[32]byte]ipc.PeerStatus)
	livePeersMu    sync.RWMutex
	triggerEngine  *triggers.Engine
	cmdExecutor    *commands.Executor
	historyManager *zdns.HistoryManager
	masterPass     string
)

func getStorage() (*zdns.Storage, error) {
	pass := masterPass
	if pass == "" {
		pass = os.Getenv("ZDNS_PASSWORD")
	}
	return zdns.NewStorage(pass)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Simple global flag check
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--pass" && i+1 < len(os.Args) {
			masterPass = os.Args[i+1]
			// Remove the flag so it doesn't break subcommands
			os.Args = append(os.Args[:i], os.Args[i+2:]...)
			break
		}
	}

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

func getHostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "UnknownDevice"
	}
	return h
}
