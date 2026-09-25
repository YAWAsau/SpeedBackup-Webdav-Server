package service

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	WindowsServiceName        = "SpeedBackupServer"
	WindowsServiceDisplayName = "SpeedBackup Server"
	// Separate variables allow isolated installer tests via build-time -X.
)

const (
	DefaultListen = "0.0.0.0:8765"
)

type WindowsPaths struct {
	InstallDir string
	DataDir    string
	BackupRoot string
}

func winJoin(base, child string) string {
	base = strings.TrimRight(base, `\/`)
	child = strings.TrimLeft(child, `\/`)
	if base == "" {
		return child
	}
	return base + `\` + child
}

func DefaultWindowsPaths(programFiles, programData string) WindowsPaths {
	install := winJoin(programFiles, "SpeedBackup Server")
	data := winJoin(programData, "SpeedBackup Server")
	return WindowsPaths{InstallDir: install, DataDir: data, BackupRoot: winJoin(data, "data")}
}

func ValidateServiceInstallArgs(exe, root, listen string) (string, error) {
	for name, v := range map[string]string{"executable": exe, "root": root, "listen": listen} {
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("%s is required", name)
		}
		if strings.Contains(v, `"`) {
			return "", fmt.Errorf("%s contains an unsupported quote", name)
		}
		if strings.ContainsAny(v, "\r\n\x00") {
			return "", fmt.Errorf("%s contains a control character", name)
		}
	}
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("invalid listen address: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid listen port %q", port)
	}
	return WindowsServiceBinaryPath(exe, root, listen), nil
}

func WindowsServiceBinaryPath(exe, root, listen string) string {
	return fmt.Sprintf(`"%s" service run --root "%s" --listen "%s"`, exe, root, listen)
}
