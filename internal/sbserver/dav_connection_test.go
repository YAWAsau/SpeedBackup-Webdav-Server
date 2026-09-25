package sbserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionIPv4Selection(t *testing.T) {
	list := []davIPv4{{IPv4: "172.20.0.1", Interface: "virtual", index: 1}, {IPv4: "192.168.0.8", Interface: "wifi", index: 2}, {IPv4: "10.0.0.4", Interface: "ethernet", index: 3}}
	for _, tc := range []struct {
		name, listen, local, route, want string
		count                            int
		loop                             bool
	}{
		{"default route", "0.0.0.0:8765", "127.0.0.1:8765", "192.168.0.8", "http://192.168.0.8:8765/", 3, false},
		{"active connection", "0.0.0.0:8765", "10.0.0.4:8765", "192.168.0.8", "http://10.0.0.4:8765/", 3, false},
		{"specific bind", "10.0.0.4:9000", "10.0.0.4:9000", "192.168.0.8", "http://10.0.0.4:9000/", 1, false},
		{"loopback bind", "127.0.0.1:8765", "127.0.0.1:8765", "192.168.0.8", "", 0, true},
		{"IPv6 loopback", "[::1]:8765", "[::1]:8765", "192.168.0.8", "", 0, true},
		{"dynamic port", ":0", "127.0.0.1:12345", "192.168.0.8", "http://192.168.0.8:12345/", 3, false},
		{"IPv6 wildcard", "[::]:8765", "[::1]:8765", "192.168.0.8", "http://192.168.0.8:8765/", 3, false},
		{"offline route", ":8765", "127.0.0.1:8765", "", "http://172.20.0.1:8765/", 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, loop := connectionIPv4(list, tc.listen, tc.local, tc.route, "http")
			if len(got) != tc.count || loop != tc.loop {
				t.Fatalf("%+v loop=%v", got, loop)
			}
			if tc.count > 0 && got[0].URL != tc.want {
				t.Fatalf("got %s want %s", got[0].URL, tc.want)
			}
		})
	}
	got, _ := connectionIPv4(nil, ":8765", "127.0.0.1:8765", "", "http")
	if got == nil || len(got) != 0 {
		t.Fatal("empty addresses must encode as []")
	}
	addresses, _ := connectionIPv4(list, ":8765", "127.0.0.1:8765", "192.168.0.8", "http")
	if confirmedConnectionURL(addresses, "127.0.0.1:8765") != "" {
		t.Fatal("default route must not be advertised as confirmed")
	}
	if confirmedConnectionURL(addresses, "10.0.0.4:8765") != "http://10.0.0.4:8765/" {
		t.Fatal("must select the actual server-side socket address")
	}
	if confirmedConnectionURL(addresses[:1], "127.0.0.1:8765") != addresses[0].URL {
		t.Fatal("single available interface should be automatic")
	}
}

func TestConnectionEndpointRequiresAdministrator(t *testing.T) {
	store, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	s := httptest.NewServer(NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte("fixture")), embeddedWeb))
	defer s.Close()
	expectStatus(t, doReq(t, s.Client(), http.MethodGet, s.URL+"/web/admin/dav_config.js", "", nil, nil), 200)
	expectStatus(t, doReq(t, s.Client(), http.MethodGet, s.URL+"/web/admin/dav_monitor.js", "", nil, nil), 200)
	expectStatus(t, doReq(t, s.Client(), http.MethodGet, s.URL+"/api/v1/admin/webdav/connection", "", nil, nil), 401)
	body := expectStatus(t, doReq(t, s.Client(), http.MethodGet, s.URL+"/api/v1/admin/webdav/connection", "fixture", nil, nil), 200)
	var result struct {
		Addresses    []davIPv4 `json:"addresses"`
		LoopbackOnly bool      `json:"loopback_only"`
		Preferred    string    `json:"preferred_url"`
	}
	if e = json.Unmarshal(body, &result); e != nil {
		t.Fatal(e)
	}
	if !result.LoopbackOnly || len(result.Addresses) != 0 || result.Preferred != "" {
		t.Fatalf("unreachable LAN advertised: %s", body)
	}
}
