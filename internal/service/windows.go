//go:build windows

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	serviceStopped            = 1
	serviceStartPending       = 2
	serviceStopPending        = 3
	serviceRunning            = 4
	serviceAcceptStop         = 0x00000001
	serviceAcceptShutdown     = 0x00000004
	serviceControlStop        = 1
	serviceControlInterrogate = 4
	serviceControlShutdown    = 5
	serviceWin32OwnProcess    = 0x00000010
)

type serviceTableEntry struct {
	name *uint16
	proc uintptr
}

type serviceStatus struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
}

var (
	advapi32                       = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcher = advapi32.NewProc("StartServiceCtrlDispatcherW")
	procRegisterServiceCtrlHandler = advapi32.NewProc("RegisterServiceCtrlHandlerExW")
	procSetServiceStatus           = advapi32.NewProc("SetServiceStatus")
	procOpenSCManager              = advapi32.NewProc("OpenSCManagerW")
	procOpenService                = advapi32.NewProc("OpenServiceW")
	procQueryServiceStatus         = advapi32.NewProc("QueryServiceStatus")
	procCloseServiceHandle         = advapi32.NewProc("CloseServiceHandle")

	callbackOnce           sync.Once
	serviceMainCallback    uintptr
	serviceControlCallback uintptr

	runMu              sync.Mutex
	activeRun          func(context.Context) error
	activeCancel       context.CancelFunc
	activeStatusHandle uintptr
	activeState        uint32
	activeAccepts      uint32
	activeRunError     error
)

func queryState() (uint32, error) {
	manager, _, err := procOpenSCManager.Call(0, 0, 1)
	if manager == 0 {
		return 0, err
	}
	defer procCloseServiceHandle.Call(manager)
	name, err := syscall.UTF16PtrFromString(WindowsServiceName)
	if err != nil {
		return 0, err
	}
	handle, _, err := procOpenService.Call(manager, uintptr(unsafe.Pointer(name)), 4)
	if handle == 0 {
		return 0, err
	}
	defer procCloseServiceHandle.Call(handle)
	var status serviceStatus
	ok, _, err := procQueryServiceStatus.Call(handle, uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return 0, err
	}
	return status.CurrentState, nil
}

func waitState(want uint32) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		state, err := queryState()
		if err != nil {
			return err
		}
		if state == want {
			return nil
		}
		if want == serviceRunning && state == serviceStopped {
			return fmt.Errorf("service stopped before becoming ready; inspect service.log")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service state %d", want)
}

func sc(args ...string) (string, error) {
	cmd := exec.Command("sc.exe", args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("sc.exe %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

func Install(exe, root, listen string) error {
	binaryPath, err := ValidateServiceInstallArgs(exe, root, listen)
	if err != nil {
		return err
	}
	if _, queryErr := queryState(); queryErr == nil {
		// Preserve an administrator's existing service account, start type and
		// recovery policy on upgrades. Only the executable command line is
		// refreshed so custom install paths/listen settings can migrate safely.
		_, err = sc("config", WindowsServiceName, "binPath=", binaryPath)
		return err
	} else if !errors.Is(queryErr, syscall.Errno(1060)) {
		return queryErr
	}
	if _, err = sc("create", WindowsServiceName, "binPath=", binaryPath, "start=", "auto", "DisplayName=", WindowsServiceDisplayName); err != nil {
		return err
	}
	if _, err = sc("description", WindowsServiceName, "SpeedBackup dedicated backup server and WebAdmin"); err != nil {
		return err
	}
	if _, err = sc("failure", WindowsServiceName, "reset=", "86400", "actions=", "restart/5000/restart/15000"); err != nil {
		return err
	}
	_, _ = sc("failureflag", WindowsServiceName, "1")
	return nil
}

func Start() error {
	state, err := queryState()
	if err != nil {
		return err
	}
	if state == serviceRunning {
		return nil
	}
	if state != serviceStartPending {
		if _, err = sc("start", WindowsServiceName); err != nil {
			return err
		}
	}
	return waitState(serviceRunning)
}

func Stop() error {
	state, err := queryState()
	if errors.Is(err, syscall.Errno(1060)) || (err == nil && state == serviceStopped) {
		return nil
	}
	if err != nil {
		return err
	}
	if state != serviceStopPending {
		if _, err = sc("stop", WindowsServiceName); err != nil {
			return err
		}
	}
	return waitState(serviceStopped)
}

func Status() (string, error) {
	return sc("query", WindowsServiceName)
}

func SetAutostart(enabled bool) error {
	mode := "demand"
	if enabled {
		mode = "auto"
	}
	_, err := sc("config", WindowsServiceName, "start=", mode)
	return err
}

func Uninstall() error {
	if _, err := queryState(); errors.Is(err, syscall.Errno(1060)) {
		return nil
	}
	if err := Stop(); err != nil {
		return err
	}
	_, err := sc("delete", WindowsServiceName)
	return err
}

func setStatus(state, accepts, checkpoint, waitHint uint32) error {
	return setStatusCode(state, accepts, checkpoint, waitHint, 0)
}

func setStatusCode(state, accepts, checkpoint, waitHint, exitCode uint32) error {
	runMu.Lock()
	h := activeStatusHandle
	activeState = state
	activeAccepts = accepts
	runMu.Unlock()
	if h == 0 {
		return fmt.Errorf("service status handle is not initialized")
	}
	s := serviceStatus{
		ServiceType:      serviceWin32OwnProcess,
		CurrentState:     state,
		ControlsAccepted: accepts,
		CheckPoint:       checkpoint,
		WaitHint:         waitHint,
		Win32ExitCode:    exitCode,
	}
	r1, _, e := procSetServiceStatus.Call(h, uintptr(unsafe.Pointer(&s)))
	if r1 == 0 {
		return e
	}
	return nil
}

func controlHandler(control, eventType, eventData, context uintptr) uintptr {
	runMu.Lock()
	cancel := activeCancel
	state := activeState
	accepts := activeAccepts
	runMu.Unlock()
	switch uint32(control) {
	case serviceControlStop, serviceControlShutdown:
		_ = setStatus(serviceStopPending, 0, 1, 10000)
		if cancel != nil {
			cancel()
		}
	case serviceControlInterrogate:
		_ = setStatus(state, accepts, 0, 0)
	}
	return 0
}

func serviceMain(argc, argv uintptr) uintptr {
	name, _ := syscall.UTF16PtrFromString(WindowsServiceName)
	r1, _, e := procRegisterServiceCtrlHandler.Call(
		uintptr(unsafe.Pointer(name)),
		serviceControlCallback,
		0,
	)
	if r1 == 0 {
		_ = e
		return 1
	}
	runMu.Lock()
	activeStatusHandle = r1
	runMu.Unlock()
	_ = setStatus(serviceStartPending, 0, 1, 10000)

	ctx, cancel := context.WithCancel(context.Background())
	runMu.Lock()
	activeCancel = cancel
	runner := activeRun
	runMu.Unlock()

	var err error
	if runner == nil {
		err = fmt.Errorf("service runner is not initialized")
	} else {
		err = runner(ctx)
	}
	cancel()
	var exitCode uint32
	if err != nil {
		exitCode = 1
	}
	_ = setStatusCode(serviceStopped, 0, 0, 0, exitCode)
	runMu.Lock()
	activeRunError = err
	activeCancel = nil
	activeRun = nil
	activeStatusHandle = 0
	runMu.Unlock()
	if err != nil {
		return 1
	}
	return 0
}

func Run(root, listen string, runner func(context.Context, io.Writer, func() error) error) error {
	if runner == nil {
		return fmt.Errorf("runner is required")
	}
	if err := os.MkdirAll(filepath.Join(root, ".speedbackup-server"), 0700); err != nil {
		return err
	}
	logPath := filepath.Join(root, ".speedbackup-server", "service.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	callbackOnce.Do(func() {
		serviceMainCallback = syscall.NewCallback(serviceMain)
		serviceControlCallback = syscall.NewCallback(controlHandler)
	})
	runMu.Lock()
	activeRunError = nil
	activeRun = func(ctx context.Context) error {
		err := runner(ctx, logFile, func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return setStatus(serviceRunning, serviceAcceptStop|serviceAcceptShutdown, 0, 0)
		})
		if err != nil {
			fmt.Fprintln(logFile, "SERVER_ERROR:", err)
			_ = logFile.Sync()
		}
		return err
	}
	runMu.Unlock()

	name, err := syscall.UTF16PtrFromString(WindowsServiceName)
	if err != nil {
		return err
	}
	table := [2]serviceTableEntry{{name: name, proc: serviceMainCallback}, {}}
	r1, _, e := procStartServiceCtrlDispatcher.Call(uintptr(unsafe.Pointer(&table[0])))
	if r1 == 0 {
		return fmt.Errorf("StartServiceCtrlDispatcherW failed: %w", e)
	}
	runMu.Lock()
	defer runMu.Unlock()
	return activeRunError
}
