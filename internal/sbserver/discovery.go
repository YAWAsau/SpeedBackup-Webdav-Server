package sbserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/brutella/dnssd"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

const davDiscoveryType = "_webdav._tcp.local."

// Application policy selects the addresses. DNS packet handling, probing,
// conflict resolution, caches and goodbye messages belong to dnssd.
func discoveryServices(tcp *net.TCPAddr, tag string) ([]dnssd.Service, string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, "", err
	}
	var services []dnssd.Service
	var keys []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		if tcp.Zone != "" && tcp.Zone != iface.Name && tcp.Zone != strconv.Itoa(iface.Index) {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		ips, _ := discoveryAddresses(tcp, addresses)
		if len(ips) == 0 {
			continue
		}
		names := make([]string, 0, len(ips))
		for _, ip := range ips {
			names = append(names, ip.String())
		}
		sort.Strings(names)
		keys = append(keys, iface.Name+"="+strings.Join(names, ","))
		// Explicit IPs are necessary: dnssd's automatic selection would include
		// addresses not accepted by an address/family-restricted TCP listener.
		service, err := dnssd.NewService(dnssd.Config{Name: "SpeedBackup-" + tag + "-" + strconv.Itoa(iface.Index), Host: "speedbackup-" + tag + "-" + strconv.Itoa(iface.Index), Type: "_webdav._tcp", Port: tcp.Port, Ifaces: []string{iface.Name}, IPs: ips, Text: map[string]string{"txtvers": "1", "path": "/", "product": "SpeedBackup", "id": tag}})
		if err != nil {
			return nil, "", err
		}
		services = append(services, service)
	}
	sort.Strings(keys)
	return services, strings.Join(keys, ";"), nil
}

func startDAVDiscovery(parent context.Context, addr net.Addr, tag string, report func(string)) func() {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || tcp.Port == 0 || tcp.IP.IsLoopback() || !validDiscoveryID(tag) {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	if report == nil {
		report = func(string) {}
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		var stop context.CancelFunc
		var ended chan error
		signature := ""
		shutdown := func() {
			if stop != nil {
				stop()
				<-ended
				stop = nil
				ended = nil
			}
		}
		defer shutdown()
		refresh := func() {
			services, key, err := discoveryServices(tcp, tag)
			if err != nil {
				report(err.Error())
				return
			}
			if key == signature && stop != nil {
				return
			}
			shutdown()
			signature = key
			if len(services) == 0 {
				return
			}
			responder, err := dnssd.NewResponder()
			if err != nil {
				report("dnssd unavailable: " + err.Error())
				return
			}
			for _, srv := range services {
				if _, err = responder.Add(srv); err != nil {
					report("dnssd add: " + err.Error())
				}
			}
			child, childCancel := context.WithCancel(ctx)
			stop = childCancel
			ended = make(chan error, 1)
			result := ended
			go func() { result <- responder.Respond(child) }()
			report(fmt.Sprintf("dnssd started port=%d interfaces=%s", tcp.Port, key))
		}
		refresh()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refresh()
			case err := <-ended:
				if err != nil && ctx.Err() == nil {
					report("dnssd stopped: " + err.Error())
				}
				if stop != nil {
					stop()
				}
				stop = nil
				ended = nil
			}
		}
	}()
	return func() { cancel(); <-done }
}

// Only addresses accepted by the TCP listener are published on their own link.
func discoveryAddresses(tcp *net.TCPAddr, addresses []net.Addr) ([]net.IP, []*net.IPNet) {
	var ips []net.IP
	var subnets []*net.IPNet
	seen := map[string]bool{}
	for _, address := range addresses {
		ip, subnet, err := net.ParseCIDR(address.String())
		if err != nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || (!ip.IsGlobalUnicast() && !ip.IsLinkLocalUnicast()) {
			continue
		}
		if tcp.IP.To4() != nil && ip.To4() == nil {
			continue
		}
		if !tcp.IP.IsUnspecified() && !tcp.IP.Equal(ip) {
			continue
		}
		if seen[ip.String()] {
			continue
		}
		seen[ip.String()] = true
		ips = append(ips, ip)
		subnets = append(subnets, subnet)
	}
	return ips, subnets
}

// Change while stopped, under the same root lock used by service startup.
func SetDiscoveryEnabled(root string, enabled bool) error {
	root, err := CleanRoot(root)
	if err != nil {
		return err
	}
	lock, err := AcquireRootLock(root)
	if err != nil {
		return err
	}
	defer lock.Close()
	store, err := NewStore(root)
	if err != nil {
		return err
	}
	defer store.diagnostics.flush()
	cfg, err := store.LoadConfig()
	if err != nil {
		return err
	}
	cfg.DiscoveryDisabled = !enabled
	return store.SaveConfig(cfg)
}

// Public installation identity, independent of authentication material.
func newDiscoveryID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
func validDiscoveryID(id string) bool {
	if len(id) != 32 || strings.ToLower(id) != id {
		return false
	}
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16
}
