//go:build !windows

package sbserver

import "errors"

func powerSupported() bool           { return false }
func launchPowerWindow(string) error { return errors.New("Windows desktop required") }
func normalPowerOff() error          { return errors.New("power off unsupported on this platform") }
