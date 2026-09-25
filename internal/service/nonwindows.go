//go:build !windows

package service

import (
	"context"
	"errors"
	"io"
)

var errWindowsOnly = errors.New("Windows service management is only available on Windows")

func Install(exe, root, listen string) error { return errWindowsOnly }
func Start() error                           { return errWindowsOnly }
func Stop() error                            { return errWindowsOnly }
func Status() (string, error)                { return "", errWindowsOnly }
func Uninstall() error                       { return errWindowsOnly }
func Run(root, listen string, runner func(context.Context, io.Writer, func() error) error) error {
	return errWindowsOnly
}
