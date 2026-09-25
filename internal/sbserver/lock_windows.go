//go:build windows

package sbserver

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type windowsRootLock struct{ h syscall.Handle }

func (l *windowsRootLock) Close() error { return syscall.CloseHandle(l.h) }

func AcquireRootLock(root string) (RootLock, error) {
	p := filepath.Join(root, ".speedbackup-server", "server.lock")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return nil, err
	}
	ptr, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return nil, err
	}
	// share mode 0 makes the open handle exclusive until process close.
	h, err := syscall.CreateFile(ptr, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("backup root is already locked by another SpeedBackup Server: %w", err)
	}
	return &windowsRootLock{h: h}, nil
}
