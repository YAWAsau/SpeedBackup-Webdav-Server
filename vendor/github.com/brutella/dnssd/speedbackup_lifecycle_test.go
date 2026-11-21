package dnssd

import (
	"context"
	"github.com/miekg/dns"
	"net"
	"sync"
	"testing"
	"time"
)

type lifecycleConn struct {
	in         chan *Request
	sent       chan struct{}
	mu         sync.Mutex
	closed     bool
	afterClose int
}

func (c *lifecycleConn) Read(context.Context) <-chan *Request { return c.in }
func (c *lifecycleConn) Drain(context.Context)                {}
func (c *lifecycleConn) SendQuery(*Query) error               { return nil }
func (c *lifecycleConn) SendResponse(*Response) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.afterClose++
	}
	select {
	case c.sent <- struct{}{}:
	default:
	}
	return nil
}
func (c *lifecycleConn) Close() { c.mu.Lock(); c.closed = true; c.mu.Unlock() }
func lifecycleFixture() (*responder, *lifecycleConn) {
	c := &lifecycleConn{in: make(chan *Request, 1), sent: make(chan struct{}, 256)}
	return newResponder(c), c
}
func TestSpeedBackupConcurrentAnnouncementsCancel(t *testing.T) {
	r, c := lifecycleFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.ctx = ctx
	s, _ := NewService(Config{Name: "test", Host: "host", Type: "_webdav._tcp", Port: 8765, IPs: []net.IP{net.IPv4(192, 0, 2, 1)}})
	iface := &net.Interface{Name: "test"}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r.announceAtInterface(&s, iface) }()
	}
	deadline := time.After(500 * time.Millisecond)
	for i := 0; i < 32; i++ {
		select {
		case <-c.sent:
		case <-deadline:
			t.Fatal("announcements serialize on the repeat delay")
		}
	}
	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("announcement cancellation did not finish promptly")
	}
	select {
	case <-c.sent:
		t.Fatal("repeat sent after cancellation")
	case <-time.After(1100 * time.Millisecond):
	}
}
func TestSpeedBackupReprobeStopsWithResponder(t *testing.T) {
	r, c := lifecycleFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, _ := NewService(Config{Name: "test", Host: "host", Type: "_webdav._tcp", Port: 8765, IPs: []net.IP{net.IPv4(192, 0, 2, 1)}})
	r.managed = []*serviceHandle{{service: &s}}
	started := make(chan struct{})
	finished := make(chan struct{})
	r.reprobeService = func(ctx context.Context, s Service) (Service, error) {
		close(started)
		<-ctx.Done()
		close(finished)
		return s, nil
	}
	done := make(chan error, 1)
	go func() { done <- r.Respond(ctx) }()
	a := &dns.A{Hdr: dns.RR_Header{Name: s.Hostname(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120}, A: net.IPv4(192, 0, 2, 2)}
	msg := new(dns.Msg)
	msg.Response = true
	msg.Answer = []dns.RR{a}
	c.in <- &Request{msg: msg, from: &net.UDPAddr{IP: net.IPv4(192, 0, 2, 2), Port: 5353}, iface: &net.Interface{Name: "test"}}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("reprobe did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reprobe outlived responder cancellation")
	}
	select {
	case <-finished:
	default:
		t.Fatal("worker still running")
	}
	r.mutex.Lock()
	remaining := len(r.managed)
	r.mutex.Unlock()
	if remaining != 0 {
		t.Fatal("cancelled reprobe re-added a service")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed || c.afterClose != 0 {
		t.Fatal("invalid socket lifecycle")
	}
}

func TestSpeedBackupQueryStaysOnSelectedInterface(t *testing.T) {
	r, c := lifecycleFixture()
	s, _ := NewService(Config{Name: "test", Host: "host", Type: "_webdav._tcp", Port: 8765, IPs: []net.IP{net.IPv4(192, 0, 2, 1)}, Ifaces: []string{"selected"}})
	m := new(dns.Msg)
	m.Question = []dns.Question{{Name: s.ServiceName(), Qtype: dns.TypePTR, Qclass: dns.ClassINET}}
	req := &Request{msg: m, from: &net.UDPAddr{IP: net.IPv4(192, 0, 2, 2), Port: 5353}, iface: &net.Interface{Name: "other"}}
	r.handleQuery(req, []*Service{&s})
	select {
	case <-c.sent:
		t.Fatal("service leaked onto an unselected interface")
	default:
	}
	req.iface = &net.Interface{Name: "selected"}
	r.handleQuery(req, []*Service{&s})
	select {
	case <-c.sent:
	default:
		t.Fatal("selected interface got no response")
	}
}

func TestSpeedBackupCancelledQueryBatch(t *testing.T) {
	r, c := lifecycleFixture()
	ctx, cancel := context.WithCancel(context.Background())
	r.ctx = ctx
	cancel()
	s, _ := NewService(Config{Name: "test", Host: "host", Type: "_webdav._tcp", Port: 8765})
	m := new(dns.Msg)
	for i := 0; i < 256; i++ {
		m.Question = append(m.Question, dns.Question{Name: s.ServiceName(), Qtype: dns.TypePTR, Qclass: dns.ClassINET})
	}
	req := &Request{msg: m, from: &net.UDPAddr{Port: 5353}, iface: &net.Interface{Name: "test"}}
	done := make(chan struct{})
	go func() { r.handleQuery(req, []*Service{&s}); close(done) }()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancelled query batch still waited for jitter")
	}
	select {
	case <-c.sent:
		t.Fatal("sent a cancelled query response")
	default:
	}
}
