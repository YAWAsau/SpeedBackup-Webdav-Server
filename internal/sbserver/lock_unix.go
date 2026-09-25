//go:build !windows

package sbserver

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func AcquireRootLock(root string) (RootLock, error) {
	p := filepath.Join(root, ".speedbackup-server", "server.lock")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("backup root is already locked by another SpeedBackup Server: %w", err)
	}
	return f, nil
}
