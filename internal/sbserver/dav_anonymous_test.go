package sbserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAnonymousSharing(t *testing.T) {
	s, _ := davTestServer(t)
	save := func(user string, anonymous, disabled bool, password string, code int) {
		b, _ := json.Marshal(map[string]any{"username": user, "anonymous": anonymous, "disabled": disabled, "password": password})
		expectStatus(t, doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/webdav", "admin", b, nil), code)
	}
	request := func(method, path, body string, headers map[string]string) *http.Response {
		r, e := http.NewRequest(method, s.URL+path, strings.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		resp, e := s.Client().Do(r)
		if e != nil {
			t.Fatal(e)
		}
		return resp
	}
	base := davPublicPrefix + "phone/"
	expectStatus(t, request("GET", base+"private", "", nil), 404)
	save("phone", true, false, "", 200)
	expectStatus(t, request("PROPFIND", strings.TrimSuffix(base, "/"), "", map[string]string{"Depth": "0"}), 207)
	expectStatus(t, davReq(t, s, "PROPFIND", "/dav", nil, map[string]string{"Depth": "0"}), 207)
	expectStatus(t, request("MKCOL", base+"Backup_zstd_0/", "", nil), 201)
	part := base + "Backup_zstd_0/user.tar.zst.part"
	dst := base + "Backup_zstd_0/user.tar.zst"
	expectStatus(t, request("PUT", part, "payload", nil), 201)
	expectStatus(t, request("HEAD", part, "", nil), 200)
	expectStatus(t, request("MOVE", part, "", map[string]string{"Destination": s.URL + dst}), 201)
	if string(expectStatus(t, request("GET", dst, "", nil), 200)) != "payload" {
		t.Fatal("anonymous content mismatch")
	}
	expectStatus(t, request("GET", dst, "", map[string]string{"Range": "bytes=1-3"}), 206)
	xml := string(expectStatus(t, request("PROPFIND", base, "", map[string]string{"Depth": "1"}), 207))
	if !strings.Contains(xml, base) {
		t.Fatal("PROPFIND lost public prefix")
	}
	expectStatus(t, request("MOVE", dst, "", map[string]string{"Destination": s.URL + davPublicPrefix + "other/stolen"}), 400)
	expectStatus(t, request("GET", base+"../other/stolen", "", nil), 403)
	expectStatus(t, request("GET", "/dav/Backup_zstd_0/user.tar.zst", "", nil), 401)
	expectStatus(t, request("GET", "/api/v1/admin/webdav", "", nil), 401)
	// Existing authenticated clients keep their password and root.
	expectStatus(t, davReq(t, s, "GET", "/dav/Backup_zstd_0/user.tar.zst", nil, nil), 200)
	save("phone", true, true, "", 200)
	expectStatus(t, request("GET", dst, "", nil), 404)
	save("phone", false, false, "", 200)
	expectStatus(t, request("GET", dst, "", nil), 404)
	// Anonymous-only share needs no password; closing it requires setting one.
	save("guest", true, false, "", 200)
	expectStatus(t, request("PROPFIND", davPublicPrefix+"guest/", "", map[string]string{"Depth": "0"}), 207)
	save("guest", false, false, "", 400)
	save("guest", false, false, "x", 200)
	expectStatus(t, request("GET", davPublicPrefix+"guest/", "", nil), 404)
	r, _ := http.NewRequest("PROPFIND", s.URL+"/dav/", nil)
	r.SetBasicAuth("guest", "x")
	r.Header.Set("Depth", "0")
	resp, e := s.Client().Do(r)
	if e != nil {
		t.Fatal(e)
	}
	expectStatus(t, resp, 207)
}
