package zdns

import (
	"encoding/json"
	"net"
)

// RegisterWithAnyisland sends a registration packet to the Anyisland daemon.
func RegisterWithAnyisland(version string) {
	packet := map[string]string{
		"op":      "REGISTER",
		"name":    "zdns",
		"source":  "github.com/nathfavour/zdns",
		"version": version,
		"type":    "binary",
	}
	
	data, err := json.Marshal(packet)
	if err != nil {
		return
	}

	// Anyisland daemon listens on UDP 1995
	conn, err := net.Dial("udp", "localhost:1995")
	if err != nil {
		return
	}
	defer conn.Close()

	conn.Write(data)
}
