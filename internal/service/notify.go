package service

import (
	"net"
	"os"
)

// NotifyReady makes Type=notify installation wait for a bound listener and
// loaded authentication config. Portable starts have no notification socket.
func NotifyReady() error {
	address := os.Getenv("NOTIFY_SOCKET")
	if address == "" {
		return nil
	}
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: address, Net: "unixgram"})
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte("READY=1\nSTATUS=Serving backup requests\n"))
	return err
}
