package sbserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func adminRequest(t *testing.T, c *http.Client, method, target string, body any, extra map[string]string) *http.Response {
	t.Helper()
	var b io.Reader
	if body != nil {
		data, e := json.Marshal(body)
		if e != nil {
			t.Fatal(e)
		}
		b = bytes.NewReader(data)
	}
	r, e := http.NewRequest(method, target, b)
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("X-SB-Admin", "1")
	r.Header.Set("Content-Type", "application/json")
	for k, v := range extra {
		r.Header.Set(k, v)
	}
	resp, e := c.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	return resp
}
func TestAdministratorLifecycle(t *testing.T) {
	root := t.TempDir()
	store, e := NewStore(root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	server := httptest.NewServer(NewHTTPHandler(store, "local", SHA256Bytes([]byte("legacy")), embeddedWeb))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	credentials := map[string]string{"username": "admin", "password": "x"}
	expectStatus(t, adminRequest(t, client, "GET", server.URL+"/api/v1/admin/directories", nil, nil), 401)
	response := adminRequest(t, client, "POST", server.URL+"/api/v1/auth/setup", credentials, nil)
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("missing protected cookie")
	}
	expectStatus(t, response, 200)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/auth/setup", credentials, nil), 409)
	expectStatus(t, adminRequest(t, client, "GET", server.URL+"/api/v1/admin/directories", nil, nil), 200)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/admin/webdav", map[string]string{"username": "phone", "password": "phone-password-123"}, map[string]string{"X-SB-Admin": ""}), 403)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/admin/webdav", map[string]string{"username": "phone", "password": "phone-password-123"}, map[string]string{"Origin": "https://attacker.example"}), 403)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/admin/password", map[string]string{"current_password": "wrong", "new_password": "y"}, nil), 401)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/admin/password", map[string]string{"current_password": "x", "new_password": "y"}, nil), 200)
	expectStatus(t, adminRequest(t, client, "GET", server.URL+"/api/v1/admin/directories", nil, nil), 401)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/auth/login", credentials, nil), 401)
	credentials["password"] = "y"
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/auth/login", credentials, nil), 200)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/auth/logout", nil, nil), 200)
	expectStatus(t, adminRequest(t, client, "GET", server.URL+"/api/v1/admin/directories", nil, nil), 401)
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	restarted := httptest.NewServer(NewHTTPHandler(store, "local", SHA256Bytes([]byte("legacy")), embeddedWeb))
	defer restarted.Close()
	expectStatus(t, adminRequest(t, client, "POST", restarted.URL+"/api/v1/auth/login", credentials, nil), 200)
	a, p, e := store.loadAdministrator()
	if e != nil || a.Username != "admin" {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if bytes.Contains(b, []byte(credentials["password"])) {
		t.Fatal("plaintext password persisted")
	}
	// Local recovery changes the account version; live sessions become invalid.
	if e = store.SetAdministrator("recovered", "z"); e != nil {
		t.Fatal(e)
	}
	expectStatus(t, adminRequest(t, client, "GET", restarted.URL+"/api/v1/admin/directories", nil, nil), 401)
}
func TestAdministratorBootstrapGuards(t *testing.T) {
	for _, tc := range []struct {
		name, remote, host, forwarded string
		code                          int
	}{
		{"remote", "192.0.2.1:123", "localhost:8765", "", 403},
		{"rebind", "127.0.0.1:123", "evil.example", "", 403},
		{"proxy", "127.0.0.1:123", "localhost", "192.0.2.1", 403},
		{"localhost", "127.0.0.1:123", "localhost", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := NewStore(t.TempDir())
			a := newAdminAuth(s)
			r := httptest.NewRequest("POST", "http://"+tc.host+"/api/v1/auth/setup", strings.NewReader(`{"username":"admin","password":"a-long-password"}`))
			r.RemoteAddr = tc.remote
			r.Header.Set("X-SB-Admin", "1")
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			w := httptest.NewRecorder()
			a.credentials(w, r)
			if w.Code != tc.code {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
	s, _ := NewStore(t.TempDir())
	a := newAdminAuth(s)
	_ = s.SetAdministrator("admin", "a-long-password")
	_, p, _ := s.loadAdministrator()
	_ = os.WriteFile(p, []byte("broken"), 0600)
	w := httptest.NewRecorder()
	a.status(w, httptest.NewRequest("GET", "http://localhost/api/v1/auth/status", nil))
	if w.Code != 500 {
		t.Fatal("corrupt account offered bootstrap")
	}
}
func TestCustomShareDirectory(t *testing.T) {
	s, root := davTestServer(t)
	external := t.TempDir()
	old := filepath.Join(root, "webdav", "phone", "old.txt")
	_ = os.WriteFile(old, []byte("old-data"), 0600)
	update := func(user, dir, password string) *http.Response {
		body, _ := json.Marshal(map[string]any{"username": user, "directory": dir, "password": password})
		return doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/webdav", "admin", body, nil)
	}
	expectStatus(t, update("phone", external, ""), 200)
	expectStatus(t, davReq(t, s, "PUT", "/dav/new.txt", strings.NewReader("new-data"), nil), 201)
	if b, e := os.ReadFile(filepath.Join(external, "new.txt")); e != nil || string(b) != "new-data" {
		t.Fatalf("custom write %s %v", b, e)
	}
	if b, e := os.ReadFile(old); e != nil || string(b) != "old-data" {
		t.Fatal("old backup was moved or deleted")
	}
	expectStatus(t, update("other", external, ""), 400)
	expectStatus(t, update("phone", root, ""), 400)
	expectStatus(t, update("phone", filepath.Join(root, ".speedbackup-server"), ""), 400)
	expectStatus(t, update("phone", "relative-path", ""), 400)
	expectStatus(t, update("phone", filepath.Join(external, "does-not-exist"), ""), 400)
	// Changing only password must retain the configured share root.
	body := []byte(`{"username":"phone","password":"correct-password-123"}`)
	expectStatus(t, doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/webdav", "admin", body, nil), 200)
	expectStatus(t, davReq(t, s, "GET", "/dav/new.txt", nil, nil), 200)
	listing := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/directories?path="+url.QueryEscape(external), "admin", nil, nil), 200)
	if !bytes.Contains(listing, []byte(`"directories"`)) {
		t.Fatal("missing directory list")
	}
	expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/directories?path="+url.QueryEscape(filepath.Join(root, ".speedbackup-server")), "admin", nil, nil), 403)
	expectStatus(t, update("phone", "", ""), 200)
	if b := expectStatus(t, davReq(t, s, "GET", "/dav/old.txt", nil, nil), 200); string(b) != "old-data" {
		t.Fatal("default directory not restored")
	}
	if _, e := os.Stat(filepath.Join(external, "new.txt")); e != nil {
		t.Fatal("custom backup was deleted")
	}
}
