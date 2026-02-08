package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type Config struct {
	SafeCommands map[string]string `json:"safe_commands"`
}

type Executor struct {
	config Config
	mu     sync.RWMutex
}

func NewExecutor(configDir string) (*Executor, error) {
	path := filepath.Join(configDir, "commands.json")
	execObj := &Executor{
		config: Config{SafeCommands: make(map[string]string)},
	}
	
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create a default commands.json with an example
		execObj.config.SafeCommands["ping"] = "echo pong"
		data, _ := json.MarshalIndent(execObj.config, "", "  ")
		os.WriteFile(path, data, 0600)
		return execObj, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &execObj.config); err != nil {
		return nil, err
	}

	return execObj, nil
}

func (e *Executor) Execute(commandName string) error {
	e.mu.RLock()
	cmdStr, ok := e.config.SafeCommands[commandName]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("command '%s' is not in the safe list", commandName)
	}

	fmt.Printf("Executing safe command: %s\n", commandName)
	cmd := exec.Command("sh", "-c", cmdStr)
	return cmd.Run()
}
