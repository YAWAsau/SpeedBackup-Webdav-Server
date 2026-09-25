//go:build !windows

package sbserver

import "path/filepath"

func davResolvePath(name string) (string, error) { return filepath.EvalSymlinks(name) }
