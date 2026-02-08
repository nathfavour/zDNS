package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nathfavour/zdns/pkg/zdns"
)

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

	fmt.Printf("%-20s %-15s %-10s %-20s
", "TIME", "PEER", "TYPE", "DATA")
	fmt.Println(strings.Repeat("-", 70))
	for _, e := range events {
		t := time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")
		fmt.Printf("%-20s %-15s %-10s %-20s
", t, e.PeerName, e.Type, e.Data)
	}
}
