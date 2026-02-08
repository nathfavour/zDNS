package sysinfo

import (
	"net"
	"strings"
	"time"
)

// ProbeTags checks each service tag (e.g. "ssh:22") and returns only the active ones.
func ProbeTags(tags string) string {
	if tags == "" {
		return ""
	}

	parts := strings.Split(tags, ",")
	active := []string{}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Split service:port
		kv := strings.Split(p, ":")
		if len(kv) != 2 {
			// If no port, just assume it's a descriptive tag and keep it
			active = append(active, p)
			continue
		}

		port := kv[1]
		// Try to connect to localhost:port
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			active = append(active, p)
		}
	}

	return strings.Join(active, ",")
}
