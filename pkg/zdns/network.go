package zdns

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	MulticastAddr = "224.0.0.251"
	MulticastPort = 5354
)

type Listener struct {
	conn      *ipv4.PacketConn
	peerStore *PeerStore
}

func NewListener(ps *PeerStore) (*Listener, error) {
	c, err := net.ListenPacket("udp4", fmt.Sprintf("0.0.0.0:%d", MulticastPort))
	if err != nil {
		return nil, err
	}

	pc := ipv4.NewPacketConn(c)
	en0, err := net.InterfaceByName("eth0") // Default to eth0 for now, should be configurable
	if err != nil {
		// Fallback to finding any up interface
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagMulticast != 0 {
				en0 = &iface
				break
			}
		}
	}

	if err := pc.JoinGroup(en0, &net.UDPAddr{IP: net.ParseIP(MulticastAddr)}); err != nil {
		return nil, err
	}

	return &Listener{
		conn:      pc,
		peerStore: ps,
	}, nil
}

func (l *Listener) Listen(handler func(peer *Peer, blob *StateBlob)) error {
	buf := make([]byte, 2048)
	for {
		n, _, _, err := l.conn.ReadFrom(buf)
		if err != nil {
			return err
		}

		if n < 16+12+16 { // Min packet size: ServiceID + Nonce + Min Ciphertext (tag)
			continue
		}

		var serviceID [16]byte
		copy(serviceID[:], buf[:16])

		peer, ok := l.peerStore.GetPeerByID(serviceID)
		if !ok {
			// Quietly drop noise
			continue
		}

		var nonce [12]byte
		copy(nonce[:], buf[16:28])
		ciphertext := buf[28:n]

		blob, err := DecryptStateBlob(peer.SharedSecret, nonce, ciphertext)
		if err != nil {
			continue
		}

		// Replay protection: simple timestamp check
		if time.Now().Unix()-blob.Timestamp > int64(MaxClockSkew.Seconds()) {
			continue
		}

		handler(peer, blob)
	}
}

type Broadcaster struct {
	conn *net.UDPConn
	addr *net.UDPAddr
}

func NewBroadcaster() (*Broadcaster, error) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", MulticastAddr, MulticastPort))
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, err
	}

	return &Broadcaster{
		conn: conn,
		addr: addr,
	}, nil
}

func (b *Broadcaster) Broadcast(peer *Peer, state DeviceState, battery uint8) error {
	blob := &StateBlob{
		DeviceID:     peer.PublicKey,
		DeviceState:  state,
		BatteryLevel: battery,
		Timestamp:    time.Now().Unix(),
	}

	// For IP, we'll just put zeros or try to find our local IP
	// In a real impl, we'd use the actual interface IP

	ciphertext, nonce, err := EncryptStateBlob(peer.SharedSecret, blob)
	if err != nil {
		return err
	}

	serviceID := GenerateServiceID(peer.SharedSecret, time.Now())

	packet := make([]byte, 16+12+len(ciphertext))
	copy(packet[0:16], serviceID[:])
	copy(packet[16:28], nonce[:])
	copy(packet[28:], ciphertext)

	_, err = b.conn.Write(packet)
	return err
}
