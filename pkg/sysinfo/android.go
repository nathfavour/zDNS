package sysinfo

import (
	"encoding/json"
	"os/exec"

	"github.com/nathfavour/zdns/pkg/zdns"
)

type AndroidProvider struct{}

type termuxBattery struct {
	Percentage int `json:"percentage"`
}

func (p *AndroidProvider) GetBatteryLevel() uint8 {
	// Requires termux-api package and app installed
	out, err := exec.Command("termux-battery-status").Output()
	if err != nil {
		return 100
	}

	var battery termuxBattery
	if err := json.Unmarshal(out, &battery); err != nil {
		return 100
	}

	return uint8(battery.Percentage)
}

func (p *AndroidProvider) GetDeviceState() zdns.DeviceState {
	// Termux doesn't have a direct way to check lock state via CLI easily without specialized tools
	// We'll return Unlocked as default for now.
	return zdns.StateUnlocked
}
