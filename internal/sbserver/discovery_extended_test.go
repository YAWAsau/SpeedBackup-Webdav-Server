package sbserver

import (
	"context"
	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"net"
	"os"
	"testing"
	"time"
)

func TestDiscoveryAddressSelectionAndAAAA(t *testing.T) {
	var addresses []net.Addr
	for _, s := range []string{"127.0.0.1/8", "192.168.1.20/24", "192.168.1.21/24", "fe80::123/64", "2001:db8::1/64", "::1/128"} {
		ip, n, _ := net.ParseCIDR(s)
		n.IP = ip
		addresses = append(addresses, n)
	}
	for _, tc := range []struct {
		bind  string
		count int
	}{{"0.0.0.0", 2}, {"::", 4}, {"192.168.1.21", 1}, {"fe80::123", 1}, {"127.0.0.1", 0}} {
		ips, _ := discoveryAddresses(&net.TCPAddr{IP: net.ParseIP(tc.bind)}, addresses)
		if len(ips) != tc.count {
			t.Fatalf("%s: %v", tc.bind, ips)
		}
	}
}

func TestDiscoveryDisablePersists(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.diagnostics.flush()
	before, _, err := s.LoadOrInitConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		if err := SetDiscoveryEnabled(root, enabled); err != nil {
			t.Fatal(err)
		}
		after, err := s.LoadConfig()
		if err != nil || after.DiscoveryDisabled == enabled || after.TokenSHA256 != before.TokenSHA256 || after.DiscoveryID != before.DiscoveryID {
			t.Fatal("setting/token persistence", err)
		}
	}
}

// Opt-in: exercises the actual multicast interface, not just loopback UDP.
func TestDiscoveryLiveLAN(t *testing.T) {
	if os.Getenv("SB_DISCOVERY_LIVE") != "1" {
		t.Skip("set SB_DISCOVERY_LIVE=1 for multicast integration")
	}
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		ips, _ := discoveryAddresses(&net.TCPAddr{IP: net.IPv4zero}, addresses)
		if len(ips) == 0 {
			continue
		}
		ln, err := net.Listen("tcp4", net.JoinHostPort(ips[0].String(), "0"))
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		stop := startDAVDiscovery(context.Background(), ln.Addr(), "1234567890abcdef1234567890abcdef", func(s string) { t.Log(s) })
		defer stop()
		client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ips[0]})
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		pc := ipv4.NewPacketConn(client)
		if err = pc.SetMulticastInterface(&iface); err != nil {
			t.Fatal(err)
		}
		_ = pc.SetMulticastLoopback(true)
		query := dnsmessage.Message{Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(davDiscoveryType), Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}}}
		packet, _ := query.Pack()
		buf := make([]byte, 9000)
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			_, _ = client.WriteToUDP(packet, (&net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}))
			_ = client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			n, _, err := client.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			var reply dnsmessage.Message
			if reply.Unpack(buf[:n]) != nil {
				continue
			}
			for _, r := range append(reply.Answers, reply.Additionals...) {
				if srv, ok := r.Body.(*dnsmessage.SRVResource); ok && int(srv.Port) == ln.Addr().(*net.TCPAddr).Port {
					t.Log("live multicast PTR/SRV resolved on", iface.Name)
					return
				}
			}
		}
		t.Fatal("no multicast response on", iface.Name)
	}
	t.Fatal("no eligible LAN interface")
}

func TestDiscoveryLiveLANIPv6(t *testing.T) {
	if os.Getenv("SB_DISCOVERY_LIVE") != "1" {
		t.Skip("set SB_DISCOVERY_LIVE=1 for multicast integration")
	}
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		ips, _ := discoveryAddresses(&net.TCPAddr{IP: net.IPv6unspecified}, addresses)
		only6 := ips[:0]
		for _, ip := range ips {
			if ip.To4() == nil {
				only6 = append(only6, ip)
			}
		}
		ips = only6
		if len(ips) == 0 {
			continue
		}
		ln, err := net.Listen("tcp6", net.JoinHostPort(ips[0].String()+"%"+iface.Name, "0"))
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		stop := startDAVDiscovery(context.Background(), ln.Addr(), "1234567890abcdef1234567890abcdef", func(s string) { t.Log(s) })
		defer stop()
		client, err := net.ListenUDP("udp6", &net.UDPAddr{IP: ips[0], Zone: iface.Name})
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		pc := ipv6.NewPacketConn(client)
		if err = pc.SetMulticastInterface(&iface); err != nil {
			t.Fatal(err)
		}
		_ = pc.SetMulticastLoopback(true)
		query := dnsmessage.Message{Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(davDiscoveryType), Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}}}
		packet, _ := query.Pack()
		buf := make([]byte, 9000)
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			_, _ = client.WriteToUDP(packet, &net.UDPAddr{IP: net.ParseIP("ff02::fb"), Port: 5353, Zone: iface.Name})
			_ = client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			n, _, err := client.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			var reply dnsmessage.Message
			if reply.Unpack(buf[:n]) != nil {
				continue
			}
			for _, r := range append(reply.Answers, reply.Additionals...) {
				if srv, ok := r.Body.(*dnsmessage.SRVResource); ok && int(srv.Port) == ln.Addr().(*net.TCPAddr).Port {
					t.Log("live multicast PTR/SRV resolved on", iface.Name)
					return
				}
			}
		}
		t.Fatal("no multicast response on", iface.Name)
	}
	t.Fatal("no eligible LAN interface")
}
