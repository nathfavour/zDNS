package zdns

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

var (
	ErrDecryptionFailed = errors.New("decryption failed: invalid key or tampered packet")
	ErrInvalidPacket    = errors.New("invalid packet format")
)

// GenerateServiceID calculates the rolling hash for a given time and secret.
func GenerateServiceID(secret []byte, t time.Time) [16]byte {
	// Calculate window index
	window := t.Unix() / int64(WindowDuration.Seconds())
	
	h := hmac.New(sha256.New, secret)
	binary.Write(h, binary.BigEndian, window)
	sum := h.Sum(nil)
	
	var id [16]byte
	copy(id[:], sum[:16])
	return id
}

// EncryptStateBlob serializes and encrypts the state blob.
func EncryptStateBlob(key []byte, blob *StateBlob) ([]byte, [12]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, [12]byte{}, err
	}

	nonce := [12]byte{}
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, [12]byte{}, err
	}

	// Simple manual serialization for speed/control
	// Base: 60 bytes
	// Type: 1 byte
	// Tags: 64 bytes
	// Cmd:  64 bytes
	// Total: 189 bytes
	data := make([]byte, 189)
	data[0] = byte(blob.Type)
	copy(data[1:33], blob.DeviceID[:])
	copy(data[33:49], blob.IP[:])
	binary.BigEndian.PutUint16(data[49:51], blob.Port)
	data[51] = byte(blob.DeviceState)
	data[52] = blob.BatteryLevel
	binary.BigEndian.PutUint64(data[53:61], uint64(blob.Timestamp))
	
	// Copy tags (max 64 bytes)
	tagBytes := []byte(blob.Tags)
	if len(tagBytes) > 64 {
		tagBytes = tagBytes[:64]
	}
	copy(data[61:125], tagBytes)

	// Copy command (max 64 bytes)
	cmdBytes := []byte(blob.Command)
	if len(cmdBytes) > 64 {
		cmdBytes = cmdBytes[:64]
	}
	copy(data[125:189], cmdBytes)

	ciphertext := aead.Seal(nil, nonce[:], data, nil)
	return ciphertext, nonce, nil
}

// DecryptStateBlob decrypts and deserializes the state blob.
func DecryptStateBlob(key []byte, nonce [12]byte, ciphertext []byte) (*StateBlob, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	plaintext, err := aead.Open(nil, nonce[:], ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	if len(plaintext) < 189 {
		return nil, ErrInvalidPacket
	}

	blob := &StateBlob{}
	blob.Type = PacketType(plaintext[0])
	copy(blob.DeviceID[:], plaintext[1:33])
	copy(blob.IP[:], plaintext[33:49])
	blob.Port = binary.BigEndian.Uint16(plaintext[49:51])
	blob.DeviceState = DeviceState(plaintext[51])
	blob.BatteryLevel = plaintext[52]
	blob.Timestamp = int64(binary.BigEndian.Uint64(plaintext[53:61]))
	
	// Extract tags and trim null bytes
	tags := string(plaintext[61:125])
	blob.Tags = strings.TrimRight(tags, "\x00")

	// Extract command and trim null bytes
	cmd := string(plaintext[125:189])
	blob.Command = strings.TrimRight(cmd, "\x00")

	return blob, nil
}
