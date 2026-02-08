package ipc

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"

	"github.com/nathfavour/zdns/pkg/zdns"
)

type Request struct {
	Command string `json:"command"`
}

type Response struct {
	Peers []PeerStatus `json:"peers,omitempty"`
	Error string       `json:"error,omitempty"`
}

type PeerStatus struct {
	Name         string `json:"name"`
	Battery      uint8  `json:"battery"`
	State        string `json:"state"`
	LastSeen     int64  `json:"last_seen"`
	PublicKey    string `json:"public_key"`
}

type Server struct {
	SocketPath string
	PeerStore  *zdns.PeerStore
	GetStatus  func() []PeerStatus // Callback to get current peer info
}

func NewServer(peerStore *zdns.PeerStore) (*Server, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	
	socketPath := filepath.Join(configDir, "zdns", "zdns.sock")
	
	// Clean up existing socket
	os.Remove(socketPath)

	return &Server{
		SocketPath: socketPath,
		PeerStore:  peerStore,
	}, nil
}

func (s *Server) Start() error {
	l, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return err
	}
	// Ensure socket is only accessible by current user
	os.Chmod(s.SocketPath, 0600)

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	
	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	var resp Response
	switch req.Command {
	case "list_peers":
		if s.GetStatus != nil {
			resp.Peers = s.GetStatus()
		}
	default:
		resp.Error = "unknown command"
	}

	json.NewEncoder(conn).Encode(resp)
}
