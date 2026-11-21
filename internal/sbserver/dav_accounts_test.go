package sbserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDAVSharedAccountQueuesOnPhysicalPath(t *testing.T) {
	d, root := newConcurrencyDAV(t)
	for _, body := range []map[string]any{{"username": "phone", "shared_directory": true}, {"username": "second", "anonymous": true, "directory": root, "shared_directory": true}} {
		b, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		d.admin(w, httptest.NewRequest("POST", "http://local/api", strings.NewReader(string(b))))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	gate, _ := d.resources("phone")
	release, err := gate.acquireNames(context.Background(), root, []string{"same.zst"})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		d.ServeHTTP(w, httptest.NewRequest("PUT", "http://local/dav-public/second/same.zst", strings.NewReader("second")))
		close(done)
	}()
	waitDAVQueue(t, gate, 1)
	release()
	awaitDAV(t, done)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	b, err := os.ReadFile(filepath.Join(root, "same.zst"))
	if err != nil || string(b) != "second" {
		t.Fatal("shared publication failed", err)
	}
}

func accountRequest(t *testing.T, s *httptest.Server, user, method, name, body string, headers map[string]string) *http.Response {
	t.Helper()
	r, _ := http.NewRequest(method, s.URL+name, strings.NewReader(body))
	r.SetBasicAuth(user, "correct-password-123")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	resp, e := s.Client().Do(r)
	if e != nil {
		t.Fatal(e)
	}
	return resp
}
func accountPost(t *testing.T, s *httptest.Server, route string, body any, status int) {
	t.Helper()
	b, e := json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	expectStatus(t, doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/webdav"+route, "admin", b, nil), status)
}
func TestDAVRenamePreservesStorageAndCredentials(t *testing.T) {
	s, root := davTestServer(t)
	expectStatus(t, accountRequest(t, s, "phone", "PUT", "/backup.zst", "original", nil), 201)
	rename := func(old, new string, status int) {
		accountPost(t, s, "/rename", map[string]string{"username": old, "new_username": new}, status)
	}
	rename("phone", "other", 409)
	rename("phone", "../escape", 400)
	rename("phone", "renamed", 200)
	expectStatus(t, accountRequest(t, s, "phone", "GET", "/backup.zst", "", nil), 401)
	if got := string(expectStatus(t, accountRequest(t, s, "renamed", "GET", "/backup.zst", "", nil), 200)); got != "original" {
		t.Fatal(got)
	}
	accountPost(t, s, "", map[string]any{"username": "renamed", "shared_directory": true}, 200)
	rename("renamed", "again", 200)
	store, e := NewStore(root)
	if e != nil {
		t.Fatal(e)
	}
	d := newDAVService(store)
	a, _, e := d.load("again")
	if e != nil || !a.SharedDirectory || !d.authenticate("again", "correct-password-123") {
		t.Fatalf("restart lost settings: %+v %v", a, e)
	}
	if d.accountDirectory(a) != filepath.Join(root, "webdav", "phone") {
		t.Fatal("directory moved")
	}
	rename("again", "phone", 200)
	expectStatus(t, accountRequest(t, s, "phone", "GET", "/backup.zst", "", nil), 200)
	if b, e := os.ReadFile(filepath.Join(root, "webdav", "phone", "backup.zst")); e != nil || string(b) != "original" {
		t.Fatal("backup changed", e)
	}
}

func TestDAVSharedDirectoryConsentAndLocks(t *testing.T) {
	s, root := davTestServer(t)
	dir := filepath.Join(root, "webdav", "phone")
	other := map[string]any{"username": "other", "directory": dir, "shared_directory": true}
	accountPost(t, s, "", other, 400)
	accountPost(t, s, "", map[string]any{"username": "phone", "shared_directory": true}, 200)
	accountPost(t, s, "", other, 200)
	expectStatus(t, accountRequest(t, s, "phone", "PUT", "/shared.zst", "shared", nil), 201)
	if b := string(expectStatus(t, accountRequest(t, s, "other", "GET", "/shared.zst", "", nil), 200)); b != "shared" {
		t.Fatal(b)
	}
	lock := accountRequest(t, s, "phone", "LOCK", "/locked.zst", `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`, map[string]string{"Depth": "0", "Timeout": "Second-60"})
	token := lock.Header.Get("Lock-Token")
	expectStatus(t, lock, 201)
	expectStatus(t, accountRequest(t, s, "other", "PUT", "/locked.zst", "blocked", nil), 423)
	expectStatus(t, accountRequest(t, s, "phone", "UNLOCK", "/locked.zst", "", map[string]string{"Lock-Token": token}), 204)
	expectStatus(t, accountRequest(t, s, "other", "PUT", "/locked.zst", "allowed", nil), 201)
	accountPost(t, s, "", map[string]any{"username": "phone", "shared_directory": false}, 400)
	child := filepath.Join(dir, "child")
	if e := os.Mkdir(child, 0700); e != nil {
		t.Fatal(e)
	}
	accountPost(t, s, "", map[string]any{"username": "child", "password": "correct-password-123", "directory": child, "shared_directory": true}, 400)
	accountPost(t, s, "", map[string]any{"username": "private", "password": "correct-password-123", "directory": filepath.Join(root, ".speedbackup-server"), "shared_directory": true}, 400)
	expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/webdav", "admin", nil, nil), 200)
}

func TestDAVRenameCustomAnonymousDisabled(t *testing.T) {
	s, root := davTestServer(t)
	dir := t.TempDir()
	accountPost(t, s, "", map[string]any{"username": "phone", "directory": dir, "anonymous": true, "disabled": true}, 200)
	accountPost(t, s, "/rename", map[string]string{"username": "phone", "new_username": "custom"}, 200)
	store, e := NewStore(root)
	if e != nil {
		t.Fatal(e)
	}
	a, _, e := newDAVService(store).load("custom")
	if e != nil || a.Directory != dir || !a.Anonymous || !a.Disabled {
		t.Fatalf("settings lost: %+v %v", a, e)
	}
}
