package sbserver

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestDiscoveryPacketAndLegacyReply(t *testing.T) {
	a, e := newDAVAdvertisement(net.IPv4(192, 168, 1, 20), 28765)
	if e != nil {
		t.Fatal(e)
	}
	q := []dnsmessage.Question{{Name: dnsmessage.MustNewName(davDiscoveryType), Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}}
	for _, legacy := range []bool{false, true} {
		var questions []dnsmessage.Question
		if legacy {
			questions = q
		}
		b, e := a.packet(123, questions, 120)
		if e != nil {
			t.Fatal(e)
		}
		var m dnsmessage.Message
		if e = m.Unpack(b); e != nil {
			t.Fatal(e)
		}
		if !m.Header.Response || !m.Header.Authoritative || len(m.Answers) != 4 {
			t.Fatalf("bad response: %+v", m)
		}
		if len(m.Questions) != len(questions) {
			t.Fatal("legacy question lost")
		}
		srv := m.Answers[1].Body.(*dnsmessage.SRVResource)
		if srv.Port != 28765 || srv.Target != a.target {
			t.Fatal("custom port/target lost")
		}
		if (uint16(m.Answers[1].Header.Class)&0x8000 == 0) != legacy {
			t.Fatal("cache flush on legacy reply")
		}
		txt := m.Answers[2].Body.(*dnsmessage.TXTResource).TXT
		if len(txt) != 3 || txt[1] != "path=/" {
			t.Fatalf("unexpected TXT: %v", txt)
		}
	}
	b, _ := a.packet(0, nil, 0)
	var goodbye dnsmessage.Message
	_ = goodbye.Unpack(b)
	for _, rr := range goodbye.Answers {
		if rr.Header.TTL != 0 {
			t.Fatal("missing goodbye")
		}
	}
	if !a.relevant(dnsmessage.Message{Questions: q}) {
		t.Fatal("PTR not matched")
	}
	q[0].Name = dnsmessage.MustNewName("_http._tcp.local.")
	if a.relevant(dnsmessage.Message{Questions: q}) {
		t.Fatal("unrelated query matched")
	}
}

func TestDiscoveryUDPQueryAndCancellation(t *testing.T) {
	server, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	client, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	a, _ := newDAVAdvertisement(net.IPv4(127, 0, 0, 1), 34567)
	_, subnet, _ := net.ParseCIDR("127.0.0.0/8")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { serveDAVDiscovery(ctx, server, subnet, a); close(done) }()
	q := dnsmessage.Message{Header: dnsmessage.Header{ID: 817}, Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(davDiscoveryType), Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}}}
	b, _ := q.Pack()
	_, e = client.WriteToUDP(b, server.LocalAddr().(*net.UDPAddr))
	if e != nil {
		t.Fatal(e)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 2048)
	n, _, e := client.ReadFromUDP(buf)
	if e != nil {
		t.Fatal(e)
	}
	var response dnsmessage.Message
	if e = response.Unpack(buf[:n]); e != nil {
		t.Fatal(e)
	}
	if response.Header.ID != 817 || len(response.Questions) != 1 {
		t.Fatal("legacy reply not echoed")
	}
	for _, rr := range response.Answers {
		if rr.Header.TTL != 10 {
			t.Fatal("legacy TTL")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("discovery cancellation stalled")
	}
}

func TestDiscoveryOPTIONSKeepsAuthentication(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.diagnostics.flush()
	server := httptest.NewServer(NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte("admin")), embeddedWeb))
	defer server.Close()
	for _, path := range []string{"/", "/dav/"} {
		req, _ := http.NewRequest("OPTIONS", server.URL+path, nil)
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != 401 || res.Header.Get("DAV") == "" || res.Header.Get("WWW-Authenticate") == "" {
			t.Fatalf("OPTIONS %s: %d %v", path, res.StatusCode, res.Header)
		}
		req, _ = http.NewRequest("PROPFIND", server.URL+path, nil)
		req.Header.Set("Depth", "0")
		res, e = server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("anonymous listing allowed: %d", res.StatusCode)
		}
	}
}
