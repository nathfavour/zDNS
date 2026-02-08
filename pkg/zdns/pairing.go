package zdns

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// PairingInvite contains the information needed to trust a new peer.
type PairingInvite struct {
	Name         string   `json:"n"`
	PublicKey    [32]byte `json:"k"`
	SharedSecret []byte   `json:"s"`
}

// CreateInvite generates a new pairing invite.
func CreateInvite(name string) (*PairingInvite, string, error) {
	// Generate a 32-byte shared secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, "", err
	}

	// For this PoC, we use a random "PublicKey" as well.
	// In a full implementation, this would be a real Ed25519 or Curve25519 key.
	pubKey := [32]byte{}
	if _, err := rand.Read(pubKey[:]); err != nil {
		return nil, "", err
	}

	invite := &PairingInvite{
		Name:         name,
		PublicKey:    pubKey,
		SharedSecret: secret,
	}

	data, err := json.Marshal(invite)
	if err != nil {
		return nil, "", err
	}

	return invite, base64.URLEncoding.EncodeToString(data), nil
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
