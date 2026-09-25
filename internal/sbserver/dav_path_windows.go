package sbserver

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var davFinalPath = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")

// Resolve through one metadata handle, including junctions and short names.
// filepath.EvalSymlinks walks every ancestor with separate Windows lookups,
// which is expensive for every operation in a deep backup directory.
// No file data is read; sharing allows concurrent read/write/rename handles.
func davResolvePath(name string) (string, error) {
	extended := filepath.Clean(name)
	if !strings.HasPrefix(extended, `\\?\`) {
		if strings.HasPrefix(extended, `\\`) {
			extended = `\\?\UNC\` + extended[2:]
		} else {
			extended = `\\?\` + extended
		}
	}
	p, err := syscall.UTF16PtrFromString(extended)
	if err != nil {
		return "", err
	}
	h, err := syscall.CreateFile(p, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", &os.PathError{Op: "resolve", Path: name, Err: err}
	}
	defer syscall.CloseHandle(h)
	buf := make([]uint16, 512)
	for {
		n, _, err := davFinalPath.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
		if n == 0 {
			// Some remote filesystems cannot provide a normalized final name.
			// Keep the standard-library behavior on those filesystems.
			_ = err
			return filepath.EvalSymlinks(name)
		}
		if n >= uintptr(len(buf)) {
			if n > 32768 {
				return "", os.ErrInvalid
			}
			buf = make([]uint16, n+1)
			continue
		}
		resolved := syscall.UTF16ToString(buf[:n])
		if strings.HasPrefix(resolved, `\\?\UNC\`) {
			return `\\` + resolved[8:], nil
		}
		return strings.TrimPrefix(resolved, `\\?\`), nil
	}
}
