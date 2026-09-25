//go:build !windows && !linux

package service

import "fmt"

func SetAutostart(enabled bool) error {
	return fmt.Errorf("automatic startup is supported on Windows and Linux")
}

func GetAutostart() (AutostartState, error) {
	return AutostartState{Mode: "unsupported", Reason: "unsupported"}, nil
}
func RunAutostartAgent() error { return fmt.Errorf("autostart agent requires Linux") }
