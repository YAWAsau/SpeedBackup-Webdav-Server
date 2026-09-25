//go:build !windows

package sbserver

func directoryRoots() []directoryEntry { return []directoryEntry{{"/", "/"}} }
