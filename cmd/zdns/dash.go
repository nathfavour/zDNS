package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/nathfavour/zdns/pkg/tui"
	"github.com/nathfavour/zdns/pkg/zdns"
)

func runDash() {
	storage, _ := getStorage()
	socketPath := filepath.Join(storage.ConfigDir, "zdns", "zdns.sock")
	if err := tui.Run(socketPath); err != nil {
		log.Fatalf("TUI Error: %v", err)
	}
}
