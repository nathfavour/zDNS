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

	if len(invite.SharedSecret) != 32 {
		return nil, fmt.Errorf("invalid secret length: expected 32, got %d", len(invite.SharedSecret))
	}

	return &invite, nil
}
