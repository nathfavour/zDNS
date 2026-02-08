package zdns

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
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
	data := make([]byte, 32+16+2+1+1+8)
	copy(data[0:32], blob.DeviceID[:])
	copy(data[32:48], blob.IP[:])
	binary.BigEndian.PutUint16(data[48:50], blob.Port)
	data[50] = byte(blob.DeviceState)
	data[51] = blob.BatteryLevel
	binary.BigEndian.PutUint64(data[52:60], uint64(blob.Timestamp))

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

	if len(plaintext) < 60 {
		return nil, ErrInvalidPacket
	}

	blob := &StateBlob{}
	copy(blob.DeviceID[:], plaintext[0:32])
	copy(blob.IP[:], plaintext[32:48])
	blob.Port = binary.BigEndian.Uint16(plaintext[48:50])
	blob.DeviceState = DeviceState(plaintext[50])
	blob.BatteryLevel = plaintext[51]
	blob.Timestamp = int64(binary.BigEndian.Uint64(plaintext[52:60]))

	return blob, nil
}
