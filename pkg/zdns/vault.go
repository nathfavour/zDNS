package zdns

import (
	"crypto/rand"
	"errors"
	"os"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

var (
	ErrInvalidPassword = errors.New("invalid master password")
)

// Vault handles encryption of data at rest.
type Vault struct {
	key [32]byte
}

// NewVault derives a 32-byte key from a password and salt.
func NewVault(password string, salt []byte) *Vault {
	// Argon2id parameters (recommended for sensitive data)
	// Time: 1, Memory: 64MB, Threads: 4
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	v := &Vault{}
	copy(v.key[:], key)
	return v
}

// Encrypt wraps plaintext with ChaCha20-Poly1305.
func (v *Vault) Encrypt(plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(v.key[:])
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// Result is nonce + ciphertext + tag
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt unwraps the vault data.
func (v *Vault) Decrypt(ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(v.key[:])
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aead.NonceSize() {
		return nil, ErrInvalidPacket
	}

	nonce, sealed := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, ErrInvalidPassword
	}

	return plaintext, nil
}

// GetOrCreateSalt ensures we have a persistent salt for key derivation.
func GetOrCreateSalt(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, salt, 0600); err != nil {
		return nil, err
	}

	return salt, nil
}
