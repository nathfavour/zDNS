package zdns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Timestamp int64  `json:"ts"`
	PeerName  string `json:"peer"`
	Type      string `json:"type"`
	Data      string `json:"data"`
}

type HistoryManager struct {
	mu        sync.RWMutex
	path      string
	events    []Event
	vault     *Vault
	configDir string
}

func NewHistoryManager(configDir string, vault *Vault) (*HistoryManager, error) {
	path := filepath.Join(configDir, "history.log")
	hm := &HistoryManager{
		path:      path,
		vault:     vault,
		configDir: configDir,
	}

	if err := hm.load(); err != nil {
		return nil, err
	}

	return hm, nil
}

func (hm *HistoryManager) load() error {
	if _, err := os.Stat(hm.path); os.IsNotExist(err) {
		hm.events = []Event{}
		return nil
	}

	data, err := os.ReadFile(hm.path)
	if err != nil {
		return err
	}

	if hm.vault != nil {
		data, err = hm.vault.Decrypt(data)
		if err != nil {
			return err
		}
	}

	return json.Unmarshal(data, &hm.events)
}

func (hm *HistoryManager) save() error {
	data, err := json.Marshal(hm.events)
	if err != nil {
		return err
	}

	if hm.vault != nil {
		data, err = hm.vault.Encrypt(data)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(hm.path, data, 0600)
}

func (hm *HistoryManager) Log(peerName, eventType, data string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	event := Event{
		Timestamp: time.Now().Unix(),
		PeerName:  peerName,
		Type:      eventType,
		Data:      data,
	}

	hm.events = append(hm.events, event)
	
	// Keep last 1000 events
	if len(hm.events) > 1000 {
		hm.events = hm.events[len(hm.events)-1000:]
	}

	hm.save()
}

func (hm *HistoryManager) GetEvents() []Event {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	// Return a copy
	out := make([]Event, len(hm.events))
	copy(out, hm.events)
	return out
}
