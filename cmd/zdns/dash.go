package main

import (
	"log"
	"path/filepath"

	"github.com/nathfavour/zdns/pkg/tui"
)

func runDash() {
	storage, _ := getStorage()
	socketPath := filepath.Join(storage.ConfigDir, "zdns", "zdns.sock")
	if err := tui.Run(socketPath); err != nil {
		log.Fatalf("TUI Error: %v", err)
	}
}
