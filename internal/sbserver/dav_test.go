package sbserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func davTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	store, e := NewStore(root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	h := NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte("admin")), embeddedWeb)
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	for _, user := range []string{"phone", "other"} {
		r := doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/webdav", "admin", []byte(fmt.Sprintf(`{"username":%q,"password":"correct-password-123"}`, user)), nil)
		expectStatus(t, r, 200)
	}
	return s, root
}
func davReq(t *testing.T, s *httptest.Server, method, name string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, e := http.NewRequest(method, s.URL+name, body)
	if e != nil {
		t.Fatal(e)
	}
	req.SetBasicAuth("phone", "correct-password-123")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, e := s.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	return resp
}
func TestDAVExistingClientWorkflow(t *testing.T) {
	for _, prefix := range []string{"/dav/", "/"} {
		t.Run(prefix, func(t *testing.T) { testDAVExistingClientWorkflow(t, prefix) })
	}
}

func testDAVExistingClientWorkflow(t *testing.T, prefix string) {
	s, root := davTestServer(t)
	dir := prefix + url.PathEscape("Backup_zstd_0") + "/"
	expectStatus(t, davReq(t, s, "MKCOL", dir, nil, nil), 201)
	dir += url.PathEscape("酷安") + "/"
	expectStatus(t, davReq(t, s, "MKCOL", dir, nil, nil), 201)
	src := dir + "user.tar.zst.part.1789810489254.13"
	dst := dir + "user.tar.zst"
	data := bytes.Repeat([]byte("stream-data"), 10000)
	// Reader without a known length makes Go use HTTP chunked transfer.
	expectStatus(t, davReq(t, s, "PUT", src, io.NopCloser(bytes.NewReader(data)), nil), 201)
	expectStatus(t, davReq(t, s, "HEAD", src, nil, nil), 200)
	expectStatus(t, davReq(t, s, "MOVE", src, nil, map[string]string{"Destination": s.URL + dst, "Overwrite": "T"}), 201)
	expectStatus(t, davReq(t, s, "HEAD", src, nil, nil), 404)
	if got := expectStatus(t, davReq(t, s, "GET", dst, nil, nil), 200); !bytes.Equal(got, data) {
		t.Fatal("content mismatch")
	}
	expectStatus(t, davReq(t, s, "GET", dst, nil, map[string]string{"Range": "bytes=1-3"}), 206)
	expectStatus(t, davReq(t, s, "PUT", src, strings.NewReader("replace"), nil), 201)
	expectStatus(t, davReq(t, s, "MOVE", src, nil, map[string]string{"Destination": s.URL + dst, "Overwrite": "F"}), 412)
	expectStatus(t, davReq(t, s, "MOVE", src, nil, map[string]string{"Destination": s.URL + dst, "Overwrite": "T"}), 204)
	expectStatus(t, davReq(t, s, "PUT", src, strings.NewReader("default-overwrite"), nil), 201)
	expectStatus(t, davReq(t, s, "MOVE", src, nil, map[string]string{"Destination": s.URL + dst}), 204)
	if got := string(expectStatus(t, davReq(t, s, "GET", dst, nil, nil), 200)); got != "default-overwrite" {
		t.Fatalf("default MOVE overwrite returned %q", got)
	}
	expectStatus(t, davReq(t, s, "COPY", dst, nil, map[string]string{"Destination": s.URL + dir + "copy", "Overwrite": "T"}), 201)
	for _, depth := range []string{"0", "1", "infinity"} {
		expectStatus(t, davReq(t, s, "PROPFIND", dir, nil, map[string]string{"Depth": depth}), 207)
	}
	// External filesystem edits must immediately appear, without rclone VFS TTL.
	physical := filepath.Join(root, "webdav", "phone", "Backup_zstd_0", "酷安", "copy")
	if e := os.Remove(physical); e != nil {
		t.Fatal(e)
	}
	listing := expectStatus(t, davReq(t, s, "PROPFIND", dir, nil, map[string]string{"Depth": "1"}), 207)
	if strings.Contains(string(listing), "/copy<") {
		t.Fatal("stale directory cache")
	}
	expectStatus(t, davReq(t, s, "DELETE", dst, nil, nil), 204)
}

type failedDAVBody struct{ sent bool }

func (b *failedDAVBody) Read(p []byte) (int, error) {
	if b.sent {
		return 0, io.ErrUnexpectedEOF
	}
	b.sent = true
	return copy(p, "partial"), nil
}
func (b *failedDAVBody) Close() error { return nil }
func TestDAVFailedPutPreservesOldFile(t *testing.T) {
	root := t.TempDir()
	store, _ := NewStore(root)
	d := newDAVService(store)
	create := httptest.NewRequest("POST", "http://local/api", strings.NewReader(`{"username":"phone","password":"correct-password-123"}`))
	cr := httptest.NewRecorder()
	d.admin(cr, create)
	if cr.Code != 200 {
		t.Fatal(cr.Body.String())
	}
	old := filepath.Join(root, "webdav", "phone", "backup.zst")
	if e := os.WriteFile(old, []byte("old-good"), 0600); e != nil {
		t.Fatal(e)
	}
	req := httptest.NewRequest("PUT", "http://local/dav/backup.zst", &failedDAVBody{})
	req.SetBasicAuth("phone", "correct-password-123")
	rr := httptest.NewRecorder()
	d.ServeHTTP(rr, req)
	if rr.Code < 400 {
		t.Fatalf("partial accepted: %d", rr.Code)
	}
	got, _ := os.ReadFile(old)
	if string(got) != "old-good" {
		t.Fatal("old backup replaced by partial")
	}
	files, _ := os.ReadDir(filepath.Dir(old))
	if len(files) != 1 {
		t.Fatal("temp file leaked")
	}
}
func TestDAVAuthIsolationAndPaths(t *testing.T) {
	s, _ := davTestServer(t)
	expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/dav/", "", nil, nil), 401)
	expectStatus(t, davReq(t, s, "PUT", "/dav//test", strings.NewReader("value"), nil), 201)
	r, _ := http.NewRequest("GET", s.URL+"/dav/test", nil)
	r.SetBasicAuth("other", "correct-password-123")
	res, e := s.Client().Do(r)
	if e != nil {
		t.Fatal(e)
	}
	expectStatus(t, res, 404)
	for _, name := range []string{"/dav/%2e%2e/secret", "/dav/..%5csecret", "/dav/.speedbackup-upload-secret", "/dav/a:ads"} {
		expectStatus(t, davReq(t, s, "GET", name, nil, nil), 403)
	}
	expectStatus(t, davReq(t, s, "MOVE", "/dav/test", nil, map[string]string{"Destination": "http://elsewhere/dav/test"}), 400)
	expectStatus(t, davReq(t, s, "DELETE", "/dav/", nil, nil), 405)
	expectStatus(t, doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/webdav", "admin", []byte(`{"username":"phone","disabled":true}`), nil), 200)
	expectStatus(t, davReq(t, s, "GET", "/dav/test", nil, nil), 401)
}
func TestDAVRenameFailurePreservesDestination(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "old"), []byte("keep"), 0600)
	rr, e := os.OpenRoot(root)
	if e != nil {
		t.Fatal(e)
	}
	defer rr.Close()
	f := &davFS{root: rr, method: "MOVE", staged: map[string]string{}}
	if e = f.RemoveAll(context.Background(), "old"); e != nil {
		t.Fatal(e)
	}
	if e = f.Rename(context.Background(), "missing", "old"); e == nil {
		t.Fatal("rename should fail")
	}
	got, _ := os.ReadFile(filepath.Join(root, "old"))
	if string(got) != "keep" {
		t.Fatal("destination lost")
	}
}

func TestDAVTransferProgressIsServerOnly(t *testing.T) {
	s, _ := davTestServer(t)
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("PUT", s.URL+"/dav/progress.part", reader)
		req.SetBasicAuth("phone", "correct-password-123")
		resp, e := s.Client().Do(req)
		if e == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 201 {
				e = fmt.Errorf("status %d", resp.StatusCode)
			}
		}
		done <- e
	}()
	if _, e := writer.Write(bytes.Repeat([]byte("x"), 64*1024)); e != nil {
		t.Fatal(e)
	}
	// Poll at most one second for the server to consume the in-flight body.
	seen := false
	for i := 0; i < 100; i++ {
		body := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/webdav/activity", "admin", nil, nil), 200)
		var state struct {
			Scope     string        `json:"scope"`
			Client    string        `json:"client_restore_status"`
			Transfers []davTransfer `json:"transfers"`
		}
		if e := json.Unmarshal(body, &state); e != nil {
			t.Fatal(e)
		}
		if state.Scope != "server_transport_only" || state.Client != "not_reported" {
			t.Fatal("incorrect client progress claim")
		}
		for _, v := range state.Transfers {
			if v.Path == "progress.part" && v.Bytes > 0 && v.State == "transferring" {
				seen = true
			}
		}
		if seen {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	writer.Close()
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if !seen {
		t.Fatal("missing live upload progress")
	}
	expectStatus(t, davReq(t, s, "GET", "/dav/progress.part", nil, nil), 200)
	body := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/webdav/activity", "admin", nil, nil), 200)
	var result struct {
		Transfers []davTransfer `json:"transfers"`
	}
	json.Unmarshal(body, &result)
	found := false
	for _, v := range result.Transfers {
		if v.Operation == "GET" {
			found = true
			if v.Bytes != 65536 || v.Expected != 65536 || v.State != "transfer_complete" {
				t.Fatalf("bad metrics: %+v", v)
			}
		}
	}
	if !found {
		t.Fatal("missing download")
	}
}
