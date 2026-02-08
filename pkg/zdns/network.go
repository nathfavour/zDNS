package zdns

import (
	"crypto/rand"
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
	
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	group := &net.UDPAddr{IP: net.ParseIP(MulticastAddr)}
	joined := 0
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		if err := pc.JoinGroup(&iface, group); err == nil {
			joined++
		}
	}

	if joined == 0 {
		return nil, fmt.Errorf("no multicast-capable interfaces found")
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

		// Security: Check for peer expiration
		if peer.ExpiresAt > 0 && time.Now().Unix() > peer.ExpiresAt {
			continue
		}

		var nonce [12]byte
		copy(nonce[:], buf[16:28])
		
		// The ciphertext is always exactly 140 bytes for our StateBlob (124 bytes + 16 tag)
		const ciphertextLen = 140
		if n < 16+12+ciphertextLen {
			continue
		}
		ciphertext := buf[28 : 28+ciphertextLen]

		blob, err := DecryptStateBlob(peer.SharedSecret, nonce, ciphertext)
		if err != nil {
			continue
		}

		// Security: Validate that the DeviceID in the blob matches the Peer
		if blob.DeviceID != peer.PublicKey {
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
	pc   *ipv4.PacketConn
	addr *net.UDPAddr
}

func NewBroadcaster() (*Broadcaster, error) {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", MulticastAddr, MulticastPort))
	if err != nil {
		return nil, err
	}

	c, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		return nil, err
	}

	return &Broadcaster{
		pc:   ipv4.NewPacketConn(c),
		addr: addr,
	}, nil
}

func (b *Broadcaster) Broadcast(peer *Peer, state DeviceState, battery uint8, tags string) error {
	blob := &StateBlob{
		DeviceID:     peer.PublicKey,
		DeviceState:  state,
		BatteryLevel: battery,
		Timestamp:    time.Now().Unix(),
		Tags:         tags,
	}

	ciphertext, nonce, err := EncryptStateBlob(peer.SharedSecret, blob)
	if err != nil {
		return err
	}

	serviceID := GenerateServiceID(peer.SharedSecret, time.Now())

	// Add random padding (0-31 bytes) to obscure packet length
	var padding [32]byte
	rand.Read(padding[:])
	paddingLen := int(padding[0] % 32)

	packet := make([]byte, 16+12+len(ciphertext)+paddingLen)
	copy(packet[0:16], serviceID[:])
	copy(packet[16:28], nonce[:])
	copy(packet[28 : 28+len(ciphertext)], ciphertext)
	copy(packet[28+len(ciphertext):], padding[:paddingLen])

	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		// Set the interface for this broadcast
		b.pc.SetMulticastInterface(&iface)
		b.pc.WriteTo(packet, nil, b.addr)
	}

	return nil
}
