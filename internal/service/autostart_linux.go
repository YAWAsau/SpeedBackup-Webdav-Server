//go:build linux

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Changing boot policy does not start or stop the current service.
func SetAutostart(enabled bool) error {
	if os.Geteuid() != 0 {
		return requestAutostartAgent(enabled)
	}
	mode := "disable"
	if enabled {
		mode = "enable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "/usr/bin/systemctl", "--no-ask-password", mode, "speedbackup-server.service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", mode, err, strings.TrimSpace(string(b)))
	}
	return nil
}

func linuxServiceProperties() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "/usr/bin/systemctl", "show", "--no-pager", "--property=LoadState,UnitFileState,MainPID,ActiveState", "speedbackup-server.service").Output()
	if err != nil {
		return nil, err
	}
	props := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			props[key] = value
		}
	}
	return props, nil
}

func GetAutostart() (AutostartState, error) {
	state := AutostartState{Supported: true, Mode: "unknown"}
	if _, err := os.Stat("/run/systemd/system"); os.IsNotExist(err) {
		state.Supported = false
		state.Mode = "unsupported"
		state.Reason = "systemd_unavailable"
		return state, nil
	}
	props, err := linuxServiceProperties()
	if err != nil {
		return state, err
	}
	if props["LoadState"] == "not-found" {
		state.Mode = "not_installed"
		state.Reason = "not_installed"
		return state, nil
	}
	state.Installed = true
	state.Mode = props["UnitFileState"]
	if state.Mode != "enabled" && state.Mode != "disabled" {
		state.Reason = "unsupported_boot_policy"
		return state, nil
	}
	enabled := state.Mode == "enabled"
	state.Enabled = &enabled
	pid, _ := strconv.Atoi(props["MainPID"])
	if pid != os.Getpid() || props["ActiveState"] != "active" {
		state.Reason = "portable"
		return state, nil
	}
	if os.Geteuid() != 0 {
		if _, err = os.Stat(autostartSocket); err != nil {
			state.Reason = "helper_unavailable"
			return state, nil
		}
	}
	state.Manageable = true
	return state, nil
}
