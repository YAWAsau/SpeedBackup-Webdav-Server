//go:build windows

package dnssd

import (
	"github.com/miekg/dns"
	"net"
	"testing"
)

func TestSpeedBackupWindowsMulticastInterface(t *testing.T) {
	c, err := newMDNSConn()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	tested := 0
	for _, iface := range MulticastInterfaces() {
		addrs, _ := iface.Addrs()
		v4 := false
		for _, a := range addrs {
			ip, _, _ := net.ParseCIDR(a.String())
			if ip.To4() != nil && !ip.IsLoopback() {
				v4 = true
			}
		}
		if !v4 || c.ipv4 == nil {
			continue
		}
		msg := new(dns.Msg)
		msg.SetQuestion("_speedbackup-test._tcp.local.", dns.TypePTR)
		if err = c.writeMsgTo(msg, iface, AddrIPv4LinkLocalMulticast); err != nil {
			t.Fatal(err)
		}
		selected, err := c.ipv4.MulticastInterface()
		if err != nil {
			t.Fatal(err)
		}
		if selected == nil || selected.Index != iface.Index {
			t.Fatalf("multicast socket did not select interface %s", iface.Name)
		}
		tested++
	}
	if tested == 0 {
		t.Skip("no IPv4 multicast interface")
	}
}
func TestSpeedBackupWindowsSharedSubnetCandidates(t *testing.T) {
	ifaces := MulticastInterfaces()
	verified := 0
	for _, origin := range ifaces {
		addrs, _ := origin.Addrs()
		for _, a := range addrs {
			ip, _, _ := net.ParseCIDR(a.String())
			if ip.To4() == nil {
				continue
			}
			actual, err := getInterfacesByIp(ip)
			if err != nil {
				t.Fatal(err)
			}
			found := map[int]bool{}
			for _, candidate := range actual {
				found[candidate.Index] = true
			}
			for _, expected := range ifaces {
				networks, _ := expected.Addrs()
				for _, network := range networks {
					_, subnet, e := net.ParseCIDR(network.String())
					if e == nil && subnet.Contains(ip) {
						if !found[expected.Index] {
							t.Fatalf("lost shared-subnet interface %s", expected.Name)
						}
						verified++
						break
					}
				}
			}
		}
	}
	if verified == 0 {
		t.Skip("no IPv4 multicast interface")
	}
}
