package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strings"

	"github.com/nathfavour/zdns/pkg/ipc"
	"github.com/nathfavour/zdns/pkg/zdns"
)

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

func runManaged() {
	fmt.Println("Checking Anyisland management status...")
	resp, err := zdns.CheckAnyislandStatus()
	if err != nil {
		fmt.Printf("Not managed by Anyisland (or Anyisland daemon not running): %v\n", err)
		return
	}

	if resp.Status == "MANAGED" {
		fmt.Println("--- Anyisland Status ---")
		fmt.Printf("Status:   %s\n", resp.Status)
		fmt.Printf("Tool ID:  %s\n", resp.ToolID)
		fmt.Printf("Version:  %s\n", resp.Version)
		fmt.Printf("AI Host:  %s\n", resp.AnyislandVersion)
		fmt.Println("------------------------")
	} else {
		fmt.Println("Status: UNMANAGED")
	}
}
