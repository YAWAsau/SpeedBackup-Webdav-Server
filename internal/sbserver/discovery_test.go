package sbserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
