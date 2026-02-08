package zdns

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RelayCheckIn is what a device sends to the signaling server.
type RelayCheckIn struct {
	IDHash    [32]byte `json:"id_hash"` // HMAC of Public Key to prevent tracking
	Encrypted []byte   `json:"data"`    // Encrypted IP/Port/State
	Nonce     [12]byte `json:"nonce"`
}

// GenerateRelayID creates a privacy-preserving hash for the relay.
func GenerateRelayID(pubKey [32]byte, secret []byte) [32]byte {
	h := hmac.New(sha256.New, secret)
	h.Write(pubKey[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// PostToRelay sends the current state to a remote signaling server.
func PostToRelay(relayURL string, pubKey [32]byte, secret []byte, blob *StateBlob) error {
	ciphertext, nonce, err := EncryptStateBlob(secret, blob)
	if err != nil {
		return err
	}

	checkIn := RelayCheckIn{
		IDHash:    GenerateRelayID(pubKey, secret),
		Encrypted: ciphertext,
		Nonce:     nonce,
	}

	data, _ := json.Marshal(checkIn)
	resp, err := http.Post(relayURL+"/checkin", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("relay returned status: %d", resp.StatusCode)
	}
	return nil
}

// QueryRelay asks the signaling server for a peer's latest state.
func QueryRelay(relayURL string, peerPubKey [32]byte, secret []byte) (*StateBlob, error) {
	idHash := GenerateRelayID(peerPubKey, secret)
	url := fmt.Sprintf("%s/lookup/%x", relayURL, idHash)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer not found on relay")
	}

	var checkIn RelayCheckIn
	if err := json.NewDecoder(resp.Body).Decode(&checkIn); err != nil {
		return nil, err
	}

	return DecryptStateBlob(secret, checkIn.Nonce, checkIn.Encrypted)
}
