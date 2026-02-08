package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/nathfavour/zdns/pkg/zdns"
)

type Identity struct {
	Name    string   `json:"name"`
	Private [32]byte `json:"priv"`
	Public  [32]byte `json:"pub"`
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
