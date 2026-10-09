//go:build windows

package sbserver

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

func powerSupported() bool { return true }
func launchPowerWindow(url string) error {
	// Launch our bundled native helper as the user, never a browser or LocalSystem UI.
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	helper := filepath.Join(filepath.Dir(executable), "speedbackup-notify.exe")
	if st, e := os.Stat(helper); e != nil || st.IsDir() {
		return errors.New("native notification helper unavailable")
	}
	args := []string{url}
	var token windows.Token
	if e := windows.WTSQueryUserToken(windows.WTSGetActiveConsoleSessionId(), &token); e == nil {
		defer token.Close()
		command := syscall.EscapeArg(helper)
		for _, a := range args {
			command += " " + syscall.EscapeArg(a)
		}
		cmd, e := windows.UTF16PtrFromString(command)
		if e != nil {
			return e
		}
		desktop, _ := windows.UTF16PtrFromString("winsta0\\default")
		si := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})), Desktop: desktop}
		var pi windows.ProcessInformation
		var env *uint16
		if e = windows.CreateEnvironmentBlock(&env, token, false); e != nil {
			return e
		}
		defer windows.DestroyEnvironmentBlock(env)
		app, _ := windows.UTF16PtrFromString(helper)
		if e = windows.CreateProcessAsUser(token, app, cmd, nil, nil, false, windows.CREATE_UNICODE_ENVIRONMENT, env, nil, &si, &pi); e != nil {
			return e
		}
		windows.CloseHandle(pi.Thread)
		windows.CloseHandle(pi.Process)
		return nil
	}
	// Portable mode may already run in the logged-in user's session.
	var session uint32
	if e := windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); e != nil || session == 0 {
		return errors.New("no interactive desktop")
	}
	cmd := exec.Command(helper, args...)
	if e := cmd.Start(); e != nil {
		return e
	}
	go cmd.Wait()
	return nil
}
func normalPowerOff() error {
	var token windows.Token
	if e := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); e != nil {
		return e
	}
	defer token.Close()
	name, _ := windows.UTF16PtrFromString("SeShutdownPrivilege")
	var id windows.LUID
	if e := windows.LookupPrivilegeValue(nil, name, &id); e != nil {
		return e
	}
	privileges := windows.Tokenprivileges{PrivilegeCount: 1}
	privileges.Privileges[0] = windows.LUIDAndAttributes{Luid: id, Attributes: windows.SE_PRIVILEGE_ENABLED}
	var previous windows.Tokenprivileges
	var size uint32
	if e := windows.AdjustTokenPrivileges(token, false, &privileges, uint32(unsafe.Sizeof(previous)), &previous, &size); e != nil {
		return e
	}
	defer windows.AdjustTokenPrivileges(token, false, &previous, 0, nil, nil)
	message, _ := windows.UTF16PtrFromString("SpeedBackup 備份完成，正在正常關機。")
	// Zero OS timeout avoids shutdown.exe's implicit force-close for nonzero /t.
	return windows.InitiateSystemShutdownEx(nil, message, 0, false, false, 0x80040000)
}
