package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nathfavour/zdns/pkg/zdns"
)

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
	fmt.Println("
" + code + "
")
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

	fmt.Printf("Successfully joined peer: %s
", invite.Name)
}
