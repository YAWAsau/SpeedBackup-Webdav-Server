//go:build linux

package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"strconv"
	"syscall"
	"time"
)

const autostartSocket = "/run/speedbackup-server-autostart.sock"

func peerCredentials(c *net.UnixConn) (*syscall.Ucred, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return nil, err
	}
	var cred *syscall.Ucred
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		cred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}
	return cred, sockErr
}

func decodeAutostartRequest(line []byte) (bool, error) {
	if len(line) > 128 {
		return false, fmt.Errorf("request too large")
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil {
		return false, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF || req.Enabled == nil {
		return false, fmt.Errorf("one enabled boolean is required")
	}
	return *req.Enabled, nil
}

func requestAutostartAgent(enabled bool) error {
	c, err := net.DialTimeout("unix", autostartSocket, 2*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(8 * time.Second))
	cred, err := peerCredentials(c.(*net.UnixConn))
	if err != nil {
		return err
	}
	if cred.Uid != 0 {
		return fmt.Errorf("invalid autostart agent owner")
	}
	if err = json.NewEncoder(c).Encode(map[string]bool{"enabled": enabled}); err != nil {
		return err
	}
	var reply struct {
		Error string `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(c, 4096)).Decode(&reply); err != nil {
		return err
	}
	if reply.Error != "" {
		return fmt.Errorf("autostart agent: %s", reply.Error)
	}
	return nil
}

// The root helper accepts one bounded boolean over a systemd-owned local socket.
// It cannot select a unit, execute a client command, or change the current service.
func RunAutostartAgent() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("autostart agent must run under systemd as root")
	}
	c, err := net.FileConn(os.Stdin)
	if err != nil {
		return err
	}
	defer c.Close()
	conn, ok := c.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("local socket required")
	}
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	cred, err := peerCredentials(conn)
	if err != nil {
		return err
	}
	if cred.Uid != 0 {
		account, e := user.Lookup("speedbackup")
		if e != nil {
			return e
		}
		uid, e := strconv.ParseUint(account.Uid, 10, 32)
		if e != nil {
			return e
		}
		props, e := linuxServiceProperties()
		if e != nil {
			return e
		}
		pid, e := strconv.ParseInt(props["MainPID"], 10, 32)
		if e != nil || cred.Uid != uint32(uid) || cred.Pid != int32(pid) || props["ActiveState"] != "active" {
			return fmt.Errorf("caller is not the running SpeedBackup service")
		}
	}
	line, err := bufio.NewReader(io.LimitReader(conn, 129)).ReadBytes('\n')
	if err != nil {
		return err
	}
	enabled, err := decodeAutostartRequest(line)
	if err == nil {
		err = SetAutostart(enabled)
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	return json.NewEncoder(conn).Encode(map[string]string{"error": message})
}
