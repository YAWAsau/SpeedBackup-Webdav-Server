package sbserver

import (
	"context"
	"encoding/json"
	"github.com/brutella/dnssd"
	"net"
	"os"
	"testing"
	"time"
)

// Invoked by scripts/check_discovery_interop.py in an isolated test process.
func TestDiscoveryInteropHarness(t *testing.T) {
	ready, stopFile := os.Getenv("SB_INTEROP_READY"), os.Getenv("SB_INTEROP_STOP")
	if ready == "" || stopFile == "" {
		t.Skip("external interoperability fixture only")
	}
	ln, err := net.Listen("tcp", "[::]:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stop func()
	if os.Getenv("SB_INTEROP_CONFLICT") == "1" {
		configs, _, err := discoveryServices(ln.Addr().(*net.TCPAddr), "1234567890abcdef1234567890abcdef")
		if err != nil || len(configs) == 0 {
			t.Fatal("no LAN", err)
		}
		srv := configs[0]
		srv.Name = "SpeedBackup-Interop"
		srv.Host = "speedbackup-interop"
		responder, err := dnssd.NewResponder()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = responder.Add(srv); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- responder.Respond(ctx) }()
		stop = func() { cancel(); <-done }
	} else {
		stop = startDAVDiscovery(ctx, ln.Addr(), "1234567890abcdef1234567890abcdef", func(message string) { t.Log(message) })
	}
	defer stop()
	body, _ := json.Marshal(map[string]any{"port": ln.Addr().(*net.TCPAddr).Port})
	if err = os.WriteFile(ready, body, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(40 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("fixture timeout")
		case <-tick.C:
			if _, err = os.Stat(stopFile); err == nil {
				return
			}
		}
	}
}
