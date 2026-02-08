package sysinfo

import (
	"os"
	"strconv"
	"strings"

	"github.com/nathfavour/zdns/pkg/zdns"
)

type LinuxProvider struct{}

func (p *LinuxProvider) GetBatteryLevel() uint8 {
	// Common paths for battery capacity on Arch/Linux
	paths := []string{
		"/sys/class/power_supply/BAT0/capacity",
		"/sys/class/power_supply/BAT1/capacity",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			val, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil {
				return uint8(val)
			}
		}
	}
	return 100 // Fallback
}

func (p *LinuxProvider) GetDeviceState() zdns.DeviceState {
	// Heuristic: Check for common screensaver/lock processes
	// In a full implementation, we'd use DBus to query logind or the session manager
	lockProcesses := []string{"swaylock", "i3lock", "gnome-screensaver", "kscreenlocker_greet"}
	
	files, err := os.ReadDir("/proc")
	if err != nil {
		return zdns.StateUnlocked
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		// Only look at PID directories
		if _, err := strconv.Atoi(file.Name()); err != nil {
			continue
		}

		comm, err := os.ReadFile("/proc/" + file.Name() + "/comm")
		if err == nil {
			name := strings.TrimSpace(string(comm))
			for _, lp := range lockProcesses {
				if name == lp {
					return zdns.StateLocked
				}
			}
		}
	}

	return zdns.StateUnlocked
}
