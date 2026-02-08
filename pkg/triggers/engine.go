package triggers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type EventType string

const (
	EventStateChange   EventType = "state_change"
	EventBatteryLevel  EventType = "battery_level"
)

// Trigger defines an action to take when an event occurs.
type Trigger struct {
	PeerName string    `json:"peer_name"`
	Event    EventType `json:"event"`
	Value    string    `json:"value"`     // e.g., "LOCKED", "UNLOCKED", "<20"
	Command  string    `json:"command"`
}

type Config struct {
	Triggers []Trigger `json:"triggers"`
}

type Engine struct {
	config Config
	mu     sync.RWMutex
}

func NewEngine(configDir string) (*Engine, error) {
	path := filepath.Join(configDir, "triggers.json")
	engine := &Engine{}
	
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create empty config if not exists
		engine.config = Config{Triggers: []Trigger{}}
		return engine, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &engine.config); err != nil {
		return nil, err
	}

	return engine, nil
}

// Execute checks if a state change matches any triggers and runs them.
func (e *Engine) CheckAndFire(peerName string, event EventType, newValue string, env map[string]string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, t := range e.config.Triggers {
		if t.PeerName == peerName && t.Event == event && t.Value == newValue {
			go e.run(t.Command, env)
		}
	}
}

func (e *Engine) run(command string, env map[string]string) {
	cmd := exec.Command("sh", "-c", command)
	
	// Set environment variables for the script
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// We fire and forget, but could log errors in a real system
	cmd.Run()
}
