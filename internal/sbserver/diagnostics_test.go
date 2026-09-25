package sbserver

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDebugExportDuringActiveUpload(t *testing.T) {
	s, _ := davTestServer(t)
	reader, writer := io.Pipe()
	defer writer.Close()
	done := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("PUT", s.URL+"/dav/ongoing.tar.zst", reader)
		req.SetBasicAuth("phone", "correct-password-123")
		response, err := s.Client().Do(req)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != 201 {
				err = fmt.Errorf("upload status %d", response.StatusCode)
			}
		}
		done <- err
	}()
	payload := bytes.Repeat([]byte("payload"), 10000)
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	// Transport buffering can accept the chunk before server authentication has
	// finished. Wait for the observable in-flight operation, not the writer alone.
	active := false
	for i := 0; i < 100; i++ {
		var state struct {
			Summary davActivityTotals `json:"summary"`
		}
		body := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/webdav/activity", "admin", nil, nil), 200)
		if err := json.Unmarshal(body, &state); err != nil {
			t.Fatal(err)
		}
		if state.Summary.Active == 1 {
			active = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !active {
		t.Fatal("upload never became active")
	}
	bundle := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/diagnostics/export", "admin", nil, nil), 200)
	var info struct {
		Activity struct {
			Summary davActivityTotals `json:"summary"`
		} `json:"activity"`
	}
	if err := json.Unmarshal(debugFiles(t, bundle)["server-info.json"], &info); err != nil {
		t.Fatal(err)
	}
	if info.Activity.Summary.Active != 1 {
		t.Fatal("missing active upload", info)
	}
	writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := expectStatus(t, davReq(t, s, "GET", "/dav/ongoing.tar.zst", nil, nil), 200); !bytes.Equal(got, payload) {
		t.Fatal("upload interrupted or altered by export")
	}
}

func debugFiles(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	out := map[string][]byte{}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		out[f.Name] = data
	}
	return out
}
func TestDebugExportContainsDetailedRequestsWithoutSecrets(t *testing.T) {
	s, root := davTestServer(t)
	secret := "NEVER_EXPORT_BODY_PASSWORD_TOKEN_COOKIE_92384"
	expectStatus(t, davReq(t, s, "PUT", "/dav/app.json", strings.NewReader(secret), nil), 201)
	expectStatus(t, davReq(t, s, "HEAD", "/dav/app.json?token="+secret, nil, map[string]string{"Cookie": "test=" + secret}), 200)
	expectStatus(t, davReq(t, s, "PROPFIND", "/dav/", nil, map[string]string{"Depth": "1"}), 207)
	expectStatus(t, davReq(t, s, "MOVE", "/dav/app.json", nil, map[string]string{"Destination": s.URL + "/dav/new.json"}), 201)
	expectStatus(t, davReq(t, s, "GET", "/dav/missing", nil, nil), 404)
	expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/native-picker/"+secret, "", nil, nil), 404)
	report := []byte(`{"page":"events","name":"ReferenceError","message":"say is not defined","stack":"at renderEvents (http://localhost/web/admin/app.js:154:9)\nhttp://localhost/?cookie=` + secret + `"}`)
	expectStatus(t, doReq(t, s.Client(), "POST", s.URL+"/api/v1/admin/diagnostics/client-error", "admin", report, nil), 200)
	expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/diagnostics/export", "", nil, nil), 401)
	body := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/diagnostics/export", "admin", nil, nil), 200)
	files := debugFiles(t, body)
	logs := string(files["logs/server.jsonl"])
	audit := string(files["audit/events.jsonl"])
	for name, data := range files {
		if strings.Contains(string(data), secret) || strings.Contains(string(data), "correct-password-123") {
			t.Fatalf("secret in %s", name)
		}
	}
	for _, method := range []string{"PUT", "HEAD", "PROPFIND", "MOVE", "GET"} {
		if !strings.Contains(logs, `"method":"`+method+`"`) {
			t.Fatalf("missing %s", method)
		}
	}
	if !strings.Contains(logs, `"name":"ReferenceError"`) || !strings.Contains(logs, "app.js:154:9") || !strings.Contains(logs, `"status":404`) || !strings.Contains(logs, `"destination":"/dav/new.json"`) {
		t.Fatal(logs)
	}
	if !strings.Contains(audit, `"request_id":`) || !strings.Contains(string(files["server-info.json"]), `"activity"`) {
		t.Fatal("missing correlation/live snapshot")
	}
	if _, e := os.Stat(filepath.Join(root, ".speedbackup-server", "logs", "server.jsonl")); e != nil {
		t.Fatal(e)
	}
}

func TestDiagnosticBatchingRotationAndPressure(t *testing.T) {
	l := &diagnosticLog{filename: filepath.Join(t.TempDir(), "server.jsonl"), limit: 300}
	defer l.flush()
	for i := 0; i < 8; i++ {
		l.append(map[string]any{"n": i, "message": strings.Repeat("a", 150)})
		if e := l.flush(); e != nil {
			t.Fatal(e)
		}
	}
	files, e := filepath.Glob(l.filename + "*")
	if e != nil || len(files) != diagnosticCopies {
		t.Fatal(files, e)
	}
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e != nil || !json.Valid(bytes.TrimSpace(b)) {
			t.Fatal(f, e)
		}
	}
	l.append(map[string]string{"too_large": strings.Repeat("b", diagnosticQueueLimit)})
	if l.status()["dropped_records"].(uint64) != 1 {
		t.Fatal(l.status())
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 25; n++ {
				l.append(map[string]int{"n": n})
			}
		}()
	}
	wg.Wait()
	if e := l.flush(); e != nil {
		t.Fatal(e)
	}
	if l.status()["pending_records"].(int) != 0 {
		t.Fatal(l.status())
	}
	// An idle logger has no permanent worker; pending records still flush promptly.
	l.append(map[string]string{"timer": "flushed"})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b, _ := os.ReadFile(l.filename); bytes.Contains(b, []byte("flushed")) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scheduled batch never reached disk")
}

func TestDiagnosticWriteFailureVisible(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if e := os.WriteFile(parent, []byte("file"), 0600); e != nil {
		t.Fatal(e)
	}
	l := &diagnosticLog{filename: filepath.Join(parent, "server.jsonl"), limit: diagnosticFileLimit}
	l.append(map[string]string{"record": "lost"})
	if e := l.flush(); e == nil {
		t.Fatal("write should fail")
	}
	if l.status()["write_errors"].(uint64) != 1 || l.status()["dropped_records"].(uint64) != 1 || l.status()["last_write_error"] == "" {
		t.Fatal(l.status())
	}
}

func TestDebugBundleBoundedTailAndAuditRotation(t *testing.T) {
	s, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	name := s.internal("audit", "events.jsonl")
	f, e := os.Create(name)
	if e != nil {
		t.Fatal(e)
	}
	// Sparse old audit file avoids allocating its full historical size.
	if _, e = f.Seek(diagnosticFileLimit+1024, io.SeekStart); e != nil {
		t.Fatal(e)
	}
	fmt.Fprintln(f, `{"unix":1,"event":"old"}`)
	f.Close()
	var b bytes.Buffer
	if e = WriteDebugBundle(s.Root, &b, nil); e != nil {
		t.Fatal(e)
	}
	files := debugFiles(t, b.Bytes())
	if len(files["audit/events.jsonl"]) > diagnosticFileLimit || !strings.Contains(string(files["bundle-manifest.json"]), `"truncated": true`) {
		t.Fatal("unbounded export")
	}
	if e = s.AppendEvent(AuditEvent{Unix: 2, Event: "new"}); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(name + ".1"); e != nil {
		t.Fatal("audit did not rotate", e)
	}
	events, e := s.TailEvents(1)
	if e != nil || len(events) != 1 || events[0].Event != "new" {
		t.Fatal(events, e)
	}
}
