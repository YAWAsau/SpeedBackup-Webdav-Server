package service

import (
	"strings"
	"testing"
)

func TestWindowsServiceBinaryPathQuotesExecutableAndRoot(t *testing.T) {
	got := WindowsServiceBinaryPath(`C:\Program Files\SpeedBackup Server\speedbackup-server.exe`, `C:\ProgramData\SpeedBackup Server\data`, "0.0.0.0:8765")
	want := `"C:\Program Files\SpeedBackup Server\speedbackup-server.exe" service run --root "C:\ProgramData\SpeedBackup Server\data" --listen "0.0.0.0:8765"`
	if got != want {
		t.Fatalf("binary path\n got: %s\nwant: %s", got, want)
	}
}

func TestWindowsServiceBinaryPathRejectsEmbeddedQuote(t *testing.T) {
	_, err := ValidateServiceInstallArgs(`C:\bad\"name.exe`, `C:\Data`, "0.0.0.0:8765")
	if err == nil || !strings.Contains(err.Error(), "quote") {
		t.Fatalf("want quote rejection, got %v", err)
	}
}

func TestDefaultServicePaths(t *testing.T) {
	p := DefaultWindowsPaths(`C:\Program Files`, `C:\ProgramData`)
	if p.InstallDir != `C:\Program Files\SpeedBackup Server` {
		t.Fatalf("install=%q", p.InstallDir)
	}
	if p.DataDir != `C:\ProgramData\SpeedBackup Server` {
		t.Fatalf("data=%q", p.DataDir)
	}
	if p.BackupRoot != `C:\ProgramData\SpeedBackup Server\data` {
		t.Fatalf("root=%q", p.BackupRoot)
	}
}

func TestValidateServiceInstallArgsRejectsBadListenPort(t *testing.T) {
	if _, err := ValidateServiceInstallArgs(`C:\Program Files\SpeedBackup Server\speedbackup-server.exe`, `C:\Data`, "0.0.0.0:99999"); err == nil {
		t.Fatal("bad port accepted")
	}
}
