package sbserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDAVRootAnonymousSelectionAndIsolation(t *testing.T) {
	s, _ := davTestServer(t)
	request := func(method, name, body string, headers map[string]string) *http.Response {
		t.Helper()
		return doReq(t, s.Client(), method, s.URL+name, "", []byte(body), headers)
	}
	save := func(user string, anonymous, disabled bool) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"username": user, "anonymous": anonymous, "disabled": disabled})
		expectStatus(t, doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/webdav", "admin", b, nil), 200)
	}
	checkRootUser := func(want string) {
		t.Helper()
		b := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/webdav", "admin", nil, nil), 200)
		var info struct {
			Path     string `json:"path"`
			RootUser string `json:"root_anonymous_user"`
		}
		if err := json.Unmarshal(b, &info); err != nil || info.Path != "/" || info.RootUser != want {
			t.Fatalf("root selection = %s, want %q", b, want)
		}
	}
	expectStatus(t, request("PROPFIND", "/", "", map[string]string{"Depth": "0"}), 401)
	save("phone", true, false)
	checkRootUser("phone")
	expectStatus(t, request("OPTIONS", "/", "", nil), 200)
	listing := string(expectStatus(t, request("PROPFIND", "/", "", map[string]string{"Depth": "0"}), 207))
	if !strings.Contains(listing, ">/</") || strings.Contains(listing, "/dav-public/") {
		t.Fatalf("root listing uses an incorrect href: %s", listing)
	}
	expectStatus(t, request("MKCOL", "/Backup_zstd_0/", "", nil), 201)
	expectStatus(t, request("MKCOL", "/Backup_zstd_0/App/", "", nil), 201)
	part := "/Backup_zstd_0/App/user.tar.zst.part"
	dest := "/Backup_zstd_0/App/user.tar.zst"
	expectStatus(t, request("PUT", "/"+part, "payload", nil), 201) // real script double slash, no redirect
	expectStatus(t, request("HEAD", part, "", nil), 200)
	expectStatus(t, request("MOVE", part, "", map[string]string{"Destination": s.URL + dest}), 201)
	expectStatus(t, request("HEAD", part, "", nil), 404)
	if string(expectStatus(t, request("GET", dest, "", nil), 200)) != "payload" {
		t.Fatal("root download mismatch")
	}
	expectStatus(t, request("GET", dest, "", map[string]string{"Range": "bytes=1-3"}), 206)
	expectStatus(t, request("GET", dest, "", map[string]string{"Authorization": "Basic Og=="}), 200)
	expectStatus(t, request("GET", "/dav-public/phone"+dest, "", nil), 200)
	expectStatus(t, davReq(t, s, "GET", "/dav"+dest, nil, nil), 200)
	for _, reserved := range []string{"api", "web", "dav", "dav-public/other"} {
		expectStatus(t, request("MOVE", dest, "", map[string]string{"Destination": s.URL + "/" + reserved + "/stolen"}), 400)
	}
	expectStatus(t, request("GET", "/api/v1/admin/webdav", "", nil), 401)
	expectStatus(t, request("PUT", "/%2e%2e/escape", "bad", nil), 403)
	expectStatus(t, request("PUT", "//api/hidden", "bad", nil), 403)
	expectStatus(t, request("DELETE", "/", "", nil), 405)
	// Bad or malformed explicit credentials never silently fall back to anonymous.
	expectStatus(t, request("GET", dest, "", map[string]string{"Authorization": "Basic invalid"}), 401)
	// Valid credentials still select their own account, even with an anonymous root.
	r, _ := http.NewRequest("GET", s.URL+dest, nil)
	r.SetBasicAuth("other", "correct-password-123")
	response, err := s.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, response, 404)
	b := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/webdav/activity", "admin", nil, nil), 200)
	var activity struct {
		Transfers []davTransfer `json:"transfers"`
	}
	if err := json.Unmarshal(b, &activity); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, transfer := range activity.Transfers {
		if transfer.Path == strings.TrimPrefix(dest, "/") && transfer.Backup == "Backup_zstd_0" && transfer.App == "App" {
			found = true
		}
	}
	if !found {
		t.Fatal("root transfer lost backup/app progress labels")
	}
	save("other", true, false)
	checkRootUser("")
	expectStatus(t, request("GET", dest, "", nil), 401)
	expectStatus(t, request("GET", "/dav-public/phone"+dest, "", nil), 200)
	save("other", true, true)
	checkRootUser("phone")
	expectStatus(t, request("GET", dest, "", nil), 200)
	save("phone", false, false)
	checkRootUser("")
	expectStatus(t, request("GET", dest, "", nil), 401)
}
