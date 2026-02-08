package zdns

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// PairingInvite contains the long-term public key for DH exchange.
type PairingInvite struct {
	Name      string   `json:"n"`
	PublicKey [32]byte `json:"k"`
}

// GenerateLongTermKey creates an X25519 keypair.
func GenerateLongTermKey() (priv, pub [32]byte, err error) {
	if _, err := rand.Read(priv[:]); err != nil {
		return priv, pub, err
	}
	curve25519.ScalarBaseMult(&pub, &priv)
	return priv, pub, nil
}

// DeriveSharedSecret performs the ECDH handshake.
func DeriveSharedSecret(myPriv, peerPub [32]byte) ([]byte, error) {
	return curve25519.X25519(myPriv[:], peerPub[:])
}

// ExportKey converts a 32-byte key to a hex string for paper backup.
func ExportKey(key [32]byte) string {
	return fmt.Sprintf("%x", key)
}

// ImportKey converts a hex string back to a 32-byte key.
func ImportKey(hexStr string) ([32]byte, error) {
	var key [32]byte
	n, err := fmt.Sscanf(hexStr, "%x", &key)
	if err != nil || n != 1 {
		return key, fmt.Errorf("invalid key format")
	}
	return key, nil
}

// ReconstitutePublic derives the public key from a private key.
func ReconstitutePublic(priv [32]byte) [32]byte {
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)
	return pub
}

// CreateInvite generates a pairing string containing the public key.
func CreateInvite(name string, pubKey [32]byte) (string, error) {
	invite := &PairingInvite{
		Name:      name,
		PublicKey: pubKey,
	}

	data, err := json.Marshal(invite)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(data), nil
}

// ParseInvite decodes a Base64 pairing string.
func ParseInvite(inviteStr string) (*PairingInvite, error) {
	data, err := base64.URLEncoding.DecodeString(inviteStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %v", err)
	}

	var invite PairingInvite
	if err := json.Unmarshal(data, &invite); err != nil {
		return nil, fmt.Errorf("invalid json: %v", err)
	}

	return &invite, nil
}