package sysinfo

import (
	"os"
	"runtime"

	"github.com/nathfavour/zdns/pkg/zdns"
)

// InfoProvider defines the interface for gathering system information.
type InfoProvider interface {
	GetBatteryLevel() uint8
	GetDeviceState() zdns.DeviceState
}

// NewInfoProvider returns the appropriate provider for the current OS.
func NewInfoProvider() InfoProvider {
	switch runtime.GOOS {
	case "linux":
		if isTermux() {
			return &AndroidProvider{}
		}
		return &LinuxProvider{}
	default:
		return &DefaultProvider{}
	}
}

func isTermux() bool {
	return os.Getenv("TERMUX_VERSION") != ""
}

// DefaultProvider returns fallback values for unsupported systems.
type DefaultProvider struct{}

func (p *DefaultProvider) GetBatteryLevel() uint8      { return 100 }
func (p *DefaultProvider) GetDeviceState() zdns.DeviceState { return zdns.StateUnlocked }
