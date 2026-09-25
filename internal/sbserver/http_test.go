package sbserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func doReq(t *testing.T, c *http.Client, method, url, token string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
func readBody(t *testing.T, r *http.Response) []byte {
	t.Helper()
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func expectStatus(t *testing.T, r *http.Response, want int) []byte {
	t.Helper()
	b := readBody(t, r)
	if r.StatusCode != want {
		t.Fatalf("status=%d want=%d body=%s", r.StatusCode, want, b)
	}
	return b
}

func TestHTTPProtocolSmoke(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token := "sb1_testtoken"
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	h := NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte(token)), nil)
	ts := httptest.NewServer(h)
	defer ts.Close()
	c := ts.Client()

	b := expectStatus(t, doReq(t, c, "GET", ts.URL+"/api/v1/capabilities", "", nil, nil), 200)
	if !strings.Contains(string(b), `"server":"SpeedBackup Server"`) {
		t.Fatalf("bad capabilities: %s", b)
	}
	expectStatus(t, doReq(t, c, "GET", ts.URL+"/api/v1/status", "", nil, nil), 401)
	expectStatus(t, doReq(t, c, "GET", ts.URL+"/api/v1/status", token, nil, nil), 200)

	create := []byte(`{"device_id":"dev1","profile_id":"default","base_generation":null}`)
	b = expectStatus(t, doReq(t, c, "POST", ts.URL+"/api/v1/sessions", token, create, nil), http.StatusCreated)
	var cr struct {
		Session SessionMeta `json:"session"`
	}
	if err := json.Unmarshal(b, &cr); err != nil {
		t.Fatal(err)
	}
	if cr.Session.ID == "" {
		t.Fatal("empty session id")
	}

	payload := []byte("SpeedBackup Server Go resumable payload 0123456789abcdefghijklmnopqrstuvwxyz")
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	first := len(payload) / 2
	p1 := payload[:first]
	p2 := payload[first:]
	u := ts.URL + "/api/v1/sessions/" + cr.Session.ID + "/objects/" + hash
	b = expectStatus(t, doReq(t, c, "PATCH", u, token, p1, map[string]string{"Content-Type": "application/octet-stream", "Upload-Offset": "0", "Upload-Length": strconv.Itoa(len(payload))}), 200)
	if !strings.Contains(string(b), `"complete":false`) {
		t.Fatalf("part1=%s", b)
	}
	r := doReq(t, c, "HEAD", u, token, nil, nil)
	expectStatus(t, r, 200)
	if r.Header.Get("Upload-Offset") != strconv.Itoa(first) {
		t.Fatalf("offset=%q", r.Header.Get("Upload-Offset"))
	}
	if r.Header.Get("Upload-Length") != strconv.Itoa(len(payload)) {
		t.Fatalf("length=%q", r.Header.Get("Upload-Length"))
	}
	b = expectStatus(t, doReq(t, c, "PATCH", u, token, p2, map[string]string{"Content-Type": "application/octet-stream", "Upload-Offset": strconv.Itoa(first), "Upload-Length": strconv.Itoa(len(payload))}), 200)
	if !strings.Contains(string(b), `"complete":true`) {
		t.Fatalf("part2=%s", b)
	}
	b = expectStatus(t, doReq(t, c, "POST", ts.URL+"/api/v1/sessions/"+cr.Session.ID+"/verify", token, nil, nil), 200)
	if !strings.Contains(string(b), `"verified":true`) {
		t.Fatalf("verify=%s", b)
	}

	commit := []byte(`{"commit_id":"c1","base_generation":null,"entries":[{"path":"smoke/payload.bin","size":` + strconv.Itoa(len(payload)) + `,"sha256":"` + hash + `","kind":"other"}]}`)
	cu := ts.URL + "/api/v1/sessions/" + cr.Session.ID + "/commit"
	b = expectStatus(t, doReq(t, c, "POST", cu, token, commit, nil), 200)
	if !strings.Contains(string(b), `"committed":true`) || !strings.Contains(string(b), `"idempotent_replay":false`) {
		t.Fatalf("commit=%s", b)
	}
	b = expectStatus(t, doReq(t, c, "POST", cu, token, commit, nil), 200)
	if !strings.Contains(string(b), `"idempotent_replay":true`) {
		t.Fatalf("replay=%s", b)
	}

	b = expectStatus(t, doReq(t, c, "GET", ts.URL+"/api/v1/manifests/current?device_id=dev1&profile_id=default", token, nil, nil), 200)
	if !strings.Contains(string(b), hash) {
		t.Fatalf("manifest=%s", b)
	}
	b = expectStatus(t, doReq(t, c, "GET", ts.URL+"/api/v1/objects/"+hash, token, nil, nil), 200)
	if !bytes.Equal(b, payload) {
		t.Fatalf("download mismatch")
	}
	b = expectStatus(t, doReq(t, c, "POST", ts.URL+"/api/v1/admin/cleanup", token, []byte(`{"dry_run":true}`), nil), 200)
	if !strings.Contains(string(b), `"dry_run":true`) {
		t.Fatalf("cleanup=%s", b)
	}
}

func TestConcurrentUploadsDoNotLoseSessionMetadata(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token := "sb1_concurrent"
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	ts := httptest.NewServer(NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte(token)), nil))
	defer ts.Close()
	b := expectStatus(t, doReq(t, ts.Client(), "POST", ts.URL+"/api/v1/sessions", token, []byte(`{"device_id":"devc","profile_id":"default","base_generation":null}`), nil), http.StatusCreated)
	var cr struct {
		Session SessionMeta `json:"session"`
	}
	if err = json.Unmarshal(b, &cr); err != nil {
		t.Fatal(err)
	}
	payloads := [][]byte{[]byte(strings.Repeat("A", 8192)), []byte(strings.Repeat("B", 12288))}
	var wg sync.WaitGroup
	errs := make(chan string, 2)
	for _, p := range payloads {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			sum := sha256.Sum256(p)
			h := hex.EncodeToString(sum[:])
			u := ts.URL + "/api/v1/sessions/" + cr.Session.ID + "/objects/" + h
			r := doReq(t, ts.Client(), "PATCH", u, token, p, map[string]string{"Content-Type": "application/octet-stream", "Upload-Offset": "0", "Upload-Length": strconv.Itoa(len(p))})
			bb := readBody(t, r)
			if r.StatusCode != 200 {
				errs <- string(bb)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("upload failed: %s", e)
	}
	b = expectStatus(t, doReq(t, ts.Client(), "GET", ts.URL+"/api/v1/sessions/"+cr.Session.ID, token, nil, nil), 200)
	var m SessionMeta
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Uploads) != 2 {
		t.Fatalf("want 2 uploads, got %d: %s", len(m.Uploads), b)
	}
}

func TestWebAdminContainsTraditionalAndSimplifiedChinese(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	ts := httptest.NewServer(NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte("x")), nil))
	defer ts.Close()
	b := expectStatus(t, doReq(t, ts.Client(), "GET", ts.URL+"/web/admin", "", nil, nil), 200)
	if !strings.Contains(string(b), "繁體中文") || !strings.Contains(string(b), "简体中文") {
		t.Fatalf("language switch missing")
	}
	b = expectStatus(t, doReq(t, ts.Client(), "GET", ts.URL+"/web/admin/app.js", "", nil, nil), 200)
	if !strings.Contains(string(b), "'zh-TW'") || !strings.Contains(string(b), "'zh-CN'") {
		t.Fatalf("i18n dictionaries missing")
	}
	icon := doReq(t, ts.Client(), "GET", ts.URL+"/web/admin/server-mark.svg", "", nil, nil)
	if !strings.Contains(icon.Header.Get("Content-Type"), "image/svg+xml") {
		t.Fatal("server icon must be served as SVG, not the index fallback")
	}
	if !strings.Contains(string(expectStatus(t, icon, 200)), "<svg") {
		t.Fatal("server icon missing")
	}
}

func TestCreateSessionRejectsStaleBaseGeneration(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Publish generation 1 to establish current state.
	m, err := BuildManifest(1, "devstale", "default", "seed", "seedcommit", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.PublishManifest(m); err != nil {
		t.Fatal(err)
	}
	token := "sb1_stale"
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	ts := httptest.NewServer(NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte(token)), nil))
	defer ts.Close()
	r := doReq(t, ts.Client(), "POST", ts.URL+"/api/v1/sessions", token, []byte(`{"device_id":"devstale","profile_id":"default","base_generation":null}`), nil)
	expectStatus(t, r, http.StatusConflict)
}

func TestHashMismatchClearsPartSoUploadCanRetry(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	token := "sb1_retry"
	t.Cleanup(func() { _ = store.diagnostics.flush() })
	ts := httptest.NewServer(NewHTTPHandler(store, "127.0.0.1:0", SHA256Bytes([]byte(token)), nil))
	defer ts.Close()
	b := expectStatus(t, doReq(t, ts.Client(), "POST", ts.URL+"/api/v1/sessions", token, []byte(`{"device_id":"devretry","profile_id":"default","base_generation":null}`), nil), http.StatusCreated)
	var cr struct {
		Session SessionMeta `json:"session"`
	}
	if err = json.Unmarshal(b, &cr); err != nil {
		t.Fatal(err)
	}
	good := []byte("correct-payload")
	bad := []byte("incorrect-data!")
	if len(good) != len(bad) {
		t.Fatal("test payload lengths differ")
	}
	sum := sha256.Sum256(good)
	h := hex.EncodeToString(sum[:])
	u := ts.URL + "/api/v1/sessions/" + cr.Session.ID + "/objects/" + h
	r := doReq(t, ts.Client(), "PATCH", u, token, bad, map[string]string{"Content-Type": "application/octet-stream", "Upload-Offset": "0", "Upload-Length": strconv.Itoa(len(good))})
	expectStatus(t, r, http.StatusConflict)
	r = doReq(t, ts.Client(), "HEAD", u, token, nil, nil)
	expectStatus(t, r, http.StatusOK)
	if got := r.Header.Get("Upload-Offset"); got != "0" {
		t.Fatalf("offset after hash mismatch=%q want 0", got)
	}
	b = expectStatus(t, doReq(t, ts.Client(), "PATCH", u, token, good, map[string]string{"Content-Type": "application/octet-stream", "Upload-Offset": "0", "Upload-Length": strconv.Itoa(len(good))}), http.StatusOK)
	if !strings.Contains(string(b), `"complete":true`) {
		t.Fatalf("retry did not complete: %s", b)
	}
}
