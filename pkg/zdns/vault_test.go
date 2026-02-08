package zdns

import (
	"bytes"
	"testing"
)

func TestVault(t *testing.T) {
	password := "super-secret-password"
	salt := []byte("1234567812345678") // 16 bytes
	
	v := NewVault(password, salt)
	
	plaintext := []byte("Hello, zDNS Vault!")
	ciphertext, err := v.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Errorf("Ciphertext should not match plaintext")
	}

	decrypted, err := v.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted data mismatch. Got %s, want %s", string(decrypted), string(plaintext))
	}

	// Test with wrong password
	v2 := NewVault("wrong-password", salt)
	_, err = v2.Decrypt(ciphertext)
	if err != ErrInvalidPassword {
		t.Errorf("Expected ErrInvalidPassword for wrong password, got %v", err)
	}
}
