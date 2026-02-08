package zdns

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"
)

type PulseHandshakeReq struct {
	Op string `json:"op"`
}

type PulseHandshakeResp struct {
	Status           string `json:"status"`
	ToolID           string `json:"tool_id"`
	Version          string `json:"version"`
	AnyislandVersion string `json:"anyisland_version"`
}

// CheckAnyislandStatus queries the Anyisland daemon to see if this tool is managed.
func CheckAnyislandStatus() (*PulseHandshakeResp, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	socketPath := filepath.Join(home, ".anyisland", "anyisland.sock")
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := PulseHandshakeReq{Op: "HANDSHAKE"}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}

	var resp PulseHandshakeResp
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
