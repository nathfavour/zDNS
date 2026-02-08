package zdns

import (
	"time"
)

// DeviceState represents the current state of the device.
type DeviceState uint8

const (
	StateLocked DeviceState = iota
	StateUnlocked
	StateDND
)

func (s DeviceState) String() string {
	switch s {
	case StateLocked:
		return "LOCKED"
	case StateUnlocked:
		return "UNLOCKED"
	case StateDND:
		return "DND"
	default:
		return "UNKNOWN"
	}
}

// StateBlob is the decrypted payload containing device information.
type StateBlob struct {
	DeviceID     [32]byte    `json:"id"`
	IP           [16]byte    `json:"ip"`
	Port         uint16      `json:"port"`
	DeviceState  DeviceState `json:"state"`
	BatteryLevel uint8       `json:"battery"`
	Timestamp    int64       `json:"ts"`
	Tags         string      `json:"tags"`
	Command      string      `json:"cmd"` // Remote command to execute
}

// Packet is the wire-format for zDNS.
// To an observer, this should look like random noise.
type Packet struct {
	// ServiceID is the rolling hash that identifies this service to peers.
	// It changes every N minutes to prevent long-term tracking.
	ServiceID [16]byte

	// Nonce for ChaCha20-Poly1305
	Nonce [12]byte

	// Ciphertext contains the encrypted and authenticated StateBlob.
	// ChaCha20-Poly1305 adds a 16-byte tag.
	Ciphertext []byte
}

const (
	// WindowDuration defines how often the ServiceID rotates.
	WindowDuration = 5 * time.Minute
	// MaxClockSkew allows for packets from slightly in the past or future.
	MaxClockSkew = 2 * WindowDuration
)
