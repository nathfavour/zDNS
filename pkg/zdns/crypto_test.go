package zdns

import (
	"testing"
	"time"
)

func TestGenerateServiceID(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-123456")
	now := time.Now()
	
	id1 := GenerateServiceID(secret, now)
	id2 := GenerateServiceID(secret, now.Add(time.Second))
	
	if id1 != id2 {
		t.Errorf("ServiceIDs in the same window should be identical")
	}

	id3 := GenerateServiceID(secret, now.Add(WindowDuration))
	if id1 == id3 {
		t.Errorf("ServiceIDs in different windows should be different")
	}
}

func TestEncryptDecryptStateBlob(t *testing.T) {
	key := []byte("key-must-be-exactly-32-bytes-len")
	blob := &StateBlob{
		DeviceID:     [32]byte{1, 2, 3},
		IP:           [16]byte{127, 0, 0, 1},
		Port:         8080,
		DeviceState:  StateUnlocked,
		BatteryLevel: 99,
		Timestamp:    time.Now().Unix(),
	}

	ciphertext, nonce, err := EncryptStateBlob(key, blob)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	decrypted, err := DecryptStateBlob(key, nonce, ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted.DeviceID != blob.DeviceID {
		t.Errorf("DeviceID mismatch")
	}
	if decrypted.BatteryLevel != blob.BatteryLevel {
		t.Errorf("BatteryLevel mismatch")
	}
	if decrypted.DeviceState != blob.DeviceState {
		t.Errorf("DeviceState mismatch")
	}
}

func TestDecryptTamperedPacket(t *testing.T) {
	key := []byte("key-must-be-exactly-32-bytes-len")
	blob := &StateBlob{BatteryLevel: 50}

	ciphertext, nonce, err := EncryptStateBlob(key, blob)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with ciphertext
	ciphertext[0] ^= 0xFF

	_, err = DecryptStateBlob(key, nonce, ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("Expected ErrDecryptionFailed, got %v", err)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := []byte("key1-must-be-exactly-32-byte-len")
	key2 := []byte("key2-must-be-exactly-32-byte-len")
	blob := &StateBlob{BatteryLevel: 50}

	ciphertext, nonce, err := EncryptStateBlob(key1, blob)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptStateBlob(key2, nonce, ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("Expected ErrDecryptionFailed with wrong key, got %v", err)
	}
}
