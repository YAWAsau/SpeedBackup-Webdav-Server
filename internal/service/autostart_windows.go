//go:build windows

package service

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func GetAutostart() (AutostartState, error) {
	state := AutostartState{Supported: true, Mode: "unknown"}
	manager, _, err := procOpenSCManager.Call(0, 0, 1)
	if manager == 0 {
		return state, err
	}
	defer procCloseServiceHandle.Call(manager)
	name, _ := syscall.UTF16PtrFromString(WindowsServiceName)
	handle, _, err := procOpenService.Call(manager, uintptr(unsafe.Pointer(name)), 1|4)
	if handle == 0 {
		if errors.Is(err, syscall.Errno(1060)) {
			state.Mode = "not_installed"
			state.Reason = "not_installed"
			return state, nil
		}
		return state, err
	}
	defer procCloseServiceHandle.Call(handle)
	state.Installed = true
	// QUERY_SERVICE_CONFIGW begins with three DWORDs; the rest is pointers.
	var buffer [8192]byte
	var needed uint32
	ok, _, err := advapi32.NewProc("QueryServiceConfigW").Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		return state, err
	}
	startType := *(*uint32)(unsafe.Pointer(&buffer[4]))
	enabled := startType == 2
	state.Enabled = &enabled
	state.Mode = "manual"
	if enabled {
		state.Mode = "enabled"
	} else if startType == 4 {
		state.Mode = "disabled"
	}
	var status struct {
		serviceStatus
		ProcessID    uint32
		ServiceFlags uint32
	}
	ok, _, err = advapi32.NewProc("QueryServiceStatusEx").Call(handle, 0, uintptr(unsafe.Pointer(&status)), unsafe.Sizeof(status), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		return state, err
	}
	state.Reason = "portable"
	if status.CurrentState == serviceRunning && status.ProcessID == uint32(os.Getpid()) {
		change, _, _ := procOpenService.Call(manager, uintptr(unsafe.Pointer(name)), 2)
		if change != 0 {
			procCloseServiceHandle.Call(change)
			state.Manageable = true
			state.Reason = ""
		} else {
			state.Reason = "permission_denied"
		}
	}
	return state, nil
}

func RunAutostartAgent() error { return syscall.ENOSYS }
