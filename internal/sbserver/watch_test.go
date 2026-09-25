package sbserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchIdleLongPollAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waiting := make(chan struct{})
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			writeJSON(w, 200, watchSnapshot{Millis: 1000, Scope: "since_server_start", Revision: "123"})
			return
		}
		if r.URL.Query().Get("watch") != "1" || r.URL.Query().Get("after") != "123" {
			t.Error("idle monitor did not request revision long poll", r.URL)
		}
		close(waiting)
		<-r.Context().Done()
	}))
	defer s.Close()
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, WatchOptions{URL: s.URL, Token: "test", Interval: 250 * time.Millisecond}, func([]string) error { return nil })
	}()
	select {
	case <-waiting:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("no idle request")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked by long poll")
	}
	if calls.Load() != 2 {
		t.Fatal("extra idle requests", calls.Load())
	}
}

func TestWatchProgressRatesStallsAndReset(t *testing.T) {
	p := watchSnapshot{Millis: 1000, Summary: davActivityTotals{UploadBytes: 1024}, Transfers: []davTransfer{{ID: 1, Started: 100, Operation: "PUT", Bytes: 1024, Expected: 4096}}}
	s := watchSnapshot{Millis: 2000, Summary: davActivityTotals{UploadBytes: 2048, Active: 1}, Transfers: []davTransfer{{ID: 1, Started: 100, Operation: "PUT", Bytes: 2048, Expected: 4096, Path: "中文\x1b[2J\n\u202e.json", Username: "phone"}}}
	frame := strings.Join(watchFrame(s, &p), "\n")
	for _, want := range []string{"50.0%", "1.0 KiB/s", "ETA 2s", "更新 JSON 列表", "中文"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("missing %s: %s", want, frame)
		}
	}
	if strings.ContainsAny(frame, "\x1b\u202e") {
		t.Fatal("terminal injection", frame)
	}
	p = s
	s.Millis += 1000
	frame = strings.Join(watchFrame(s, &p), "\n")
	if !strings.Contains(frame, "0.0 B/s") || !strings.Contains(frame, "ETA --") {
		t.Fatal("stalled transfer must show zero and unknown ETA", frame)
	}
	s.Transfers[0].Expected = -1
	if frame = strings.Join(watchFrame(s, &p), "\n"); !strings.Contains(frame, "大小未知") {
		t.Fatal(frame)
	}
	s.Transfers[0].Expected = s.Transfers[0].Bytes
	if frame = strings.Join(watchFrame(s, &p), "\n"); !strings.Contains(frame, "99.9%") {
		t.Fatal("must not claim success before request finishes", frame)
	}
	s.Summary.UploadBytes = 0
	s.Transfers[0].Started = 1900
	if frame = strings.Join(watchFrame(s, &p), "\n"); strings.Contains(frame, "/s") {
		t.Fatal("reset or first sample must have no inferred speed", frame)
	}
}

func TestWatchURLAndRedirectCredentialBoundary(t *testing.T) {
	for _, bad := range []string{"http://example.com", "ftp://localhost", "http://u:p@localhost", "http://localhost/x", "http://localhost?token=x", "http://localhost/#x"} {
		if _, e := WatchURL(bad); e == nil {
			t.Fatal("accepted", bad)
		}
	}
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	err := Watch(context.Background(), WatchOptions{URL: source.URL, Username: "admin", Password: "secret", Interval: time.Second, Once: true}, func([]string) error { return nil })
	if err == nil || leaked.Load() || strings.Contains(err.Error(), "secret") {
		t.Fatal("redirect or secret leak", err)
	}
}

func TestWatchLoginLogoutAndExpiry(t *testing.T) {
	store, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.diagnostics.flush()
	if e = store.SetAdministrator("admin", "test-only"); e != nil {
		t.Fatal(e)
	}
	h := NewHTTPHandler(store, "local", SHA256Bytes([]byte("legacy")), embeddedWeb)
	var logins, logouts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			logins.Add(1)
		}
		if r.URL.Path == "/api/v1/auth/logout" {
			logouts.Add(1)
		}
		h.ServeHTTP(w, r)
	}))
	defer server.Close()
	calls := 0
	e = Watch(context.Background(), WatchOptions{URL: server.URL, Username: "admin", Password: "test-only", Interval: time.Second, Once: true}, func(lines []string) error {
		calls++
		if !strings.Contains(strings.Join(lines, "\n"), "等待傳輸") {
			t.Error(lines)
		}
		return nil
	})
	if e != nil || calls != 1 || logins.Load() != 1 || logouts.Load() != 1 {
		t.Fatal(e, calls, logins.Load(), logouts.Load())
	}
	e = Watch(context.Background(), WatchOptions{URL: server.URL, Token: "wrong", Interval: time.Second, Once: true}, func([]string) error { t.Error("unauthenticated frame"); return nil })
	if e == nil || !strings.Contains(e.Error(), "permission") {
		t.Fatal(e)
	}
}

func TestWatchReconnectAndWriterFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode(watchSnapshot{Millis: time.Now().UnixMilli(), Scope: "since_server_start"})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	frames := 0
	e := Watch(ctx, WatchOptions{URL: server.URL, Token: "token", Interval: 250 * time.Millisecond}, func(lines []string) error {
		frames++
		if frames == 1 && !strings.Contains(strings.Join(lines, "\n"), "重試") {
			t.Error(lines)
		}
		if frames == 2 {
			cancel()
		}
		return nil
	})
	if e != nil || frames != 2 {
		t.Fatal(e, frames)
	}
	want := errors.New("closed output")
	e = Watch(context.Background(), WatchOptions{URL: server.URL, Token: "token", Interval: time.Second, Once: true}, func([]string) error { return want })
	if !errors.Is(e, want) {
		t.Fatal(e)
	}
}

func TestWatchExitDoesNotInterruptActiveWebDAVUpload(t *testing.T) {
	server, _ := davTestServer(t)
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	payload := bytes.Repeat([]byte("independent-transfer"), 8192)
	req, _ := http.NewRequest("PUT", server.URL+"/watch.bin", reader)
	req.ContentLength = int64(len(payload))
	req.SetBasicAuth("phone", "correct-password-123")
	transferDone := make(chan error, 1)
	go func() {
		resp, e := server.Client().Do(req)
		if e == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 201 {
				e = errors.New("PUT did not complete")
			}
		}
		transferDone <- e
	}()
	if _, e := writer.Write(payload[:65536]); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	seen := false
	e := Watch(ctx, WatchOptions{URL: server.URL, Token: "admin", Interval: 250 * time.Millisecond}, func(lines []string) error {
		frame := strings.Join(lines, "\n")
		if strings.Contains(frame, "watch.bin") && strings.Contains(frame, "ETA") {
			seen = true
			cancel()
		}
		return nil
	})
	if e != nil || !seen {
		t.Fatal("no live upload observed", e)
	}
	select {
	case e := <-transferDone:
		t.Fatal("upload ended before viewer exit", e)
	default:
	}
	if _, e = writer.Write(payload[65536:]); e != nil {
		t.Fatal(e)
	}
	writer.Close()
	select {
	case e = <-transferDone:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("upload blocked by viewer")
	}
	resp := davReq(t, server, "GET", "/watch.bin", nil, nil)
	got, e := io.ReadAll(resp.Body)
	resp.Body.Close()
	if e != nil || sha256.Sum256(got) != sha256.Sum256(payload) {
		t.Fatal("content changed", e)
	}
}
