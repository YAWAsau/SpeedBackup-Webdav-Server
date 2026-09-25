//go:build windows

package sbserver

import (
	"fmt"
	"syscall"
)

// Enumerate drive letters without touching disconnected network drives.
func directoryRoots() []directoryEntry {
	mask, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetLogicalDrives").Call()
	out := []directoryEntry{}
	for i := 0; i < 26; i++ {
		if mask&(1<<i) != 0 {
			p := fmt.Sprintf("%c:\\", 'A'+i)
			out = append(out, directoryEntry{p, p})
		}
	}
	return out
}
