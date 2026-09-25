package sbserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
)

const davDiscoveryType = "_webdav._tcp.local."

var davDiscoveryGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

type davAdvertisement struct {
	instance, target dnsmessage.Name
	ip               [4]byte
	port             uint16
}

// Every listener instance has a fresh 96-bit name; no user, filesystem path,
// account, token or password is published. HTTP independently verifies hints.
func newDAVAdvertisement(ip net.IP, port int) (davAdvertisement, error) {
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		return davAdvertisement{}, err
	}
	tag := hex.EncodeToString(id[:])
	a := davAdvertisement{port: uint16(port), instance: dnsmessage.MustNewName("SpeedBackup-" + tag + "." + davDiscoveryType), target: dnsmessage.MustNewName("speedbackup-" + tag + ".local.")}
	copy(a.ip[:], ip.To4())
	return a, nil
}

func (a davAdvertisement) packet(id uint16, questions []dnsmessage.Question, ttl uint32) ([]byte, error) {
	// Legacy replies repeat the original question and must not set cache-flush.
	cls := dnsmessage.ClassINET
	if len(questions) == 0 {
		cls = dnsmessage.Class(uint16(cls) | 0x8000)
	}
	header := func(n dnsmessage.Name, t dnsmessage.Type, c dnsmessage.Class) dnsmessage.ResourceHeader {
		return dnsmessage.ResourceHeader{Name: n, Type: t, Class: c, TTL: ttl}
	}
	m := dnsmessage.Message{Header: dnsmessage.Header{ID: id, Response: true, Authoritative: true}, Questions: questions, Answers: []dnsmessage.Resource{
		{Header: header(dnsmessage.MustNewName(davDiscoveryType), dnsmessage.TypePTR, dnsmessage.ClassINET), Body: &dnsmessage.PTRResource{PTR: a.instance}},
		{Header: header(a.instance, dnsmessage.TypeSRV, cls), Body: &dnsmessage.SRVResource{Port: a.port, Target: a.target}},
		{Header: header(a.instance, dnsmessage.TypeTXT, cls), Body: &dnsmessage.TXTResource{TXT: []string{"txtvers=1", "path=/", "product=SpeedBackup"}}},
		{Header: header(a.target, dnsmessage.TypeA, cls), Body: &dnsmessage.AResource{A: a.ip}},
	}}
	return m.Pack()
}

func (a davAdvertisement) relevant(m dnsmessage.Message) bool {
	if m.Header.Response || len(m.Questions) > 16 {
		return false
	}
	for _, q := range m.Questions {
		if uint16(q.Class)&0x7fff != uint16(dnsmessage.ClassINET) {
			continue
		}
		n := strings.ToLower(q.Name.String())
		if n == davDiscoveryType && (q.Type == dnsmessage.TypePTR || q.Type == dnsmessage.TypeALL) {
			return true
		}
		if n == strings.ToLower(a.instance.String()) && (q.Type == dnsmessage.TypeSRV || q.Type == dnsmessage.TypeTXT || q.Type == dnsmessage.TypeALL) {
			return true
		}
		if n == a.target.String() && (q.Type == dnsmessage.TypeA || q.Type == dnsmessage.TypeALL) {
			return true
		}
	}
	return false
}

// Optional discovery starts only after TCP bind/auth initialization. Failure to
// join multicast never prevents the ordinary WebDAV service from starting.
func startDAVDiscovery(parent context.Context, addr net.Addr, report func(string)) func() {
	ctx, cancel := context.WithCancel(parent)
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || tcp.Port == 0 || tcp.IP.IsLoopback() {
		return cancel
	}
	var wg sync.WaitGroup
	if report == nil {
		report = func(string) {}
	}
	type membership struct {
		stop    context.CancelFunc
		done    chan struct{}
		address string
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		live := map[int]membership{}
		defer func() {
			for _, entry := range live {
				entry.stop()
			}
		}()
		refresh := func() {
			interfaces, err := net.Interfaces()
			if err != nil {
				return
			}
			wanted := map[int]bool{}
			for _, iface := range interfaces {
				if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
					continue
				}
				addresses, _ := iface.Addrs()
				for _, address := range addresses {
					ip, subnet, err := net.ParseCIDR(address.String())
					if err != nil || ip.To4() == nil || (!ip.IsPrivate() && !ip.IsLinkLocalUnicast()) {
						continue
					}
					if !tcp.IP.IsUnspecified() && !tcp.IP.Equal(ip) {
						continue
					}
					wanted[iface.Index] = true
					if entry, exists := live[iface.Index]; exists {
						alive := true
						select {
						case <-entry.done:
							alive = false
						default:
						}
						if alive && entry.address == address.String() {
							break
						}
						entry.stop()
						<-entry.done
						delete(live, iface.Index)
					}
					socket, err := net.ListenMulticastUDP("udp4", &iface, davDiscoveryGroup)
					if err != nil {
						report(fmt.Sprintf("interface=%s unavailable=%v", iface.Name, err))
						break
					}
					pc := ipv4.NewPacketConn(socket)
					if err = pc.SetMulticastInterface(&iface); err == nil {
						err = pc.SetMulticastTTL(255)
					}
					if err != nil {
						socket.Close()
						report(fmt.Sprintf("interface=%s setup=%v", iface.Name, err))
						break
					}
					_ = pc.SetMulticastLoopback(true)
					advert, err := newDAVAdvertisement(ip, tcp.Port)
					if err != nil {
						socket.Close()
						break
					}
					child, stop := context.WithCancel(ctx)
					done := make(chan struct{})
					live[iface.Index] = membership{stop: stop, done: done, address: address.String()}
					wg.Add(1)
					go func() { defer wg.Done(); defer close(done); serveDAVDiscovery(child, socket, subnet, advert) }()
					report(fmt.Sprintf("interface=%s address=%s port=%d state=advertising", iface.Name, ip, tcp.Port))
					break
				}
			}
			for index, entry := range live {
				if !wanted[index] {
					entry.stop()
					delete(live, index)
				}
			}
		}
		refresh()
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				refresh()
			}
		}
	}()
	return func() { cancel(); wg.Wait() }
}

func serveDAVDiscovery(ctx context.Context, socket *net.UDPConn, subnet *net.IPNet, a davAdvertisement) {
	defer socket.Close()
	announce := func(ttl uint32) {
		if packet, e := a.packet(0, nil, ttl); e == nil {
			_, _ = socket.WriteToUDP(packet, davDiscoveryGroup)
		}
	}
	defer announce(0)
	announce(120)
	next := time.Now().Add(time.Second)
	buf := make([]byte, 9000)
	var lastReply time.Time
	for {
		if ctx.Err() != nil {
			return
		}
		if time.Now().After(next) {
			announce(120)
			next = time.Now().Add(60 * time.Second)
		}
		_ = socket.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, peer, err := socket.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
		if !subnet.Contains(peer.IP) || time.Since(lastReply) < 25*time.Millisecond {
			continue
		}
		var m dnsmessage.Message
		if err = m.Unpack(buf[:n]); err != nil || !a.relevant(m) {
			continue
		}
		destination := davDiscoveryGroup
		id := uint16(0)
		var questions []dnsmessage.Question
		ttl := uint32(120)
		if peer.Port != 5353 {
			destination = peer
			id = m.Header.ID
			questions = m.Questions
			ttl = 10
		} else {
			for _, q := range m.Questions {
				if uint16(q.Class)&0x8000 != 0 {
					destination = peer
					break
				}
			}
		}
		packet, err := a.packet(id, questions, ttl)
		if err == nil {
			_, _ = socket.WriteToUDP(packet, destination)
			lastReply = time.Now()
		}
	}
}
