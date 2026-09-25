package service

import (
	"fmt"
	"sync"
)

type AutostartState struct {
	Supported  bool   `json:"supported"`
	Installed  bool   `json:"installed"`
	Enabled    *bool  `json:"enabled"`
	Manageable bool   `json:"manageable"`
	Mode       string `json:"mode"`
	Reason     string `json:"reason,omitempty"`
}

var autostartMu sync.Mutex

// Only the running installed service may change its own boot policy via HTTP.
// A separate portable instance must never control an unrelated installation.
func UpdateAutostart(enabled bool) (AutostartState, error) {
	autostartMu.Lock()
	defer autostartMu.Unlock()
	state, err := GetAutostart()
	if err != nil {
		return state, err
	}
	if !state.Manageable {
		return state, fmt.Errorf("autostart cannot be changed: %s", state.Reason)
	}
	if err = SetAutostart(enabled); err != nil {
		return state, err
	}
	state, err = GetAutostart()
	if err == nil && (state.Enabled == nil || *state.Enabled != enabled) {
		err = fmt.Errorf("autostart read-back did not match the requested setting")
	}
	return state, err
}
