package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/nathfavour/zdns/pkg/zdns"
)

func runRelayServer() {
	fs := flag.NewFlagSet("relay", flag.ExitOnError)
	port := fs.String("port", "8080", "Port to listen on")
	fs.Parse(os.Args[2:])

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
