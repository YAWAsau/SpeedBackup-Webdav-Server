package sbserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPowerRoutesAdminAndCredentialIsolation(t *testing.T) {
	store, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	handler := NewHTTPHandler(store, "127.0.0.1:8765", SHA256Bytes([]byte("legacy-token")), embeddedWeb)
	defer handler.(*managedHTTPHandler).stop()
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	expectStatus(t, adminRequest(t, client, "GET", server.URL+"/api/v1/admin/power", nil, map[string]string{"Authorization": "Bearer legacy-token"}), 401)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/auth/setup", map[string]string{"username": "admin", "password": "test-password"}, nil), 200)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/admin/power", map[string]string{"action": "rotate"}, map[string]string{"Origin": "https://evil.example"}), 403)
	response := adminRequest(t, client, "POST", server.URL+"/api/v1/admin/power", map[string]string{"action": "rotate"}, nil)
	var v struct {
		Token   string `json:"token"`
		Enabled bool   `json:"enabled"`
	}
	e = json.NewDecoder(response.Body).Decode(&v)
	response.Body.Close()
	if e != nil || v.Token == "" || v.Enabled {
		t.Fatal(e, v)
	}
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/power/complete", map[string]string{"job_id": "unique-backup-job-999", "result": "success"}, nil), 401)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/power/complete", map[string]string{"job_id": "unique-backup-job-999", "result": "success"}, map[string]string{"Authorization": "Bearer " + v.Token}), 409)
	expectStatus(t, adminRequest(t, client, "POST", server.URL+"/api/v1/admin/power", map[string]any{"action": "theme", "theme": map[string]string{"theme": "evil"}}, nil), 400)
	for _, asset := range []string{"power_settings.js"} {
		expectStatus(t, adminRequest(t, client, "GET", server.URL+"/web/admin/"+asset, nil, nil), 200)
	}
}

func powerFixture(t *testing.T) *powerManager {
	t.Helper()
	store, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p := newPowerManager(store)
	p.supported = true
	p.config.Enabled = true
	p.config.TokenHash = SHA256Bytes([]byte("test-power-token"))
	p.launch = func(string) error { return nil }
	p.shutdown = func() error { t.Fatal("unexpected shutdown"); return nil }
	t.Cleanup(p.close)
	return p
}
func TestPowerWaitPauseDeferCancel(t *testing.T) {
	p := powerFixture(t)
	now := time.Now()
	p.job = &powerJob{ID: "job", State: "waiting", created: now}
	p.tick(now)
	if p.job.Deadline != 0 {
		t.Fatal("countdown without desktop")
	}
	p.job.heartbeat = now
	p.active = 1
	p.tick(now)
	if p.job.Deadline != 0 {
		t.Fatal("countdown during transfer")
	}
	p.active = 0
	p.tick(now)
	if p.job.Deadline != now.Add(time.Minute).UnixMilli() {
		t.Fatal(p.job)
	}
	p.tick(now.Add(11 * time.Second))
	if p.job.Deadline != 0 {
		t.Fatal("offline desktop must reset countdown")
	}
	p.job.heartbeat = now
	p.tick(now)
	if !p.begin() {
		t.Fatal("admission")
	}
	if p.job.Deadline != 0 {
		t.Fatal("new transfer must pause countdown")
	}
	p.end()
	if e := p.action("defer", "wrong-job"); e == nil {
		t.Fatal("stale action accepted")
	}
	if e := p.action("defer", "job"); e != nil {
		t.Fatal(e)
	}
	p.tick(now)
	if p.job.State != "deferred" {
		t.Fatal(p.job)
	}
	if e := p.action("cancel", "job"); e != nil {
		t.Fatal(e)
	}
	p.tick(now.Add(time.Hour))
	if p.job.State != "cancelled" {
		t.Fatal(p.job)
	}
}

func TestPowerManualRequiresConfirmationAndPreviewCannotEscalate(t *testing.T) {
	p := powerFixture(t)
	p.config.Enabled = false
	s := &HTTPServer{power: p, listen: "127.0.0.1:8765"}
	call := func(body string) int {
		r := httptest.NewRequest("POST", "/api/v1/power/manual", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer test-power-token")
		w := httptest.NewRecorder()
		s.powerComplete(w, r)
		return w.Code
	}
	body := `{"job_id":"manual-failed-job-123","operation":"restore","outcome":"failed","user_confirmed":true,"preview":true}`
	if code := call(strings.Replace(body, `"user_confirmed":true`, `"user_confirmed":false`, 1)); code != 400 {
		t.Fatal(code)
	}
	if code := call(strings.Replace(body, `"preview":true`, `"preview":false`, 1)); code != 409 {
		t.Fatal(code)
	}
	if code := call(body); code != 202 || !p.job.Preview || p.job.Outcome != "failed" || p.job.Operation != "restore" {
		t.Fatal(code, p.job)
	}
	p.mu.Lock()
	p.job.heartbeat = time.Now()
	p.job.Deadline = time.Now().UnixMilli()
	p.tick(time.Now())
	p.mu.Unlock()
	if p.job.State != "simulated" {
		t.Fatal(p.job)
	}
	if code := call(strings.Replace(body, `"preview":true`, `"preview":false`, 1)); code != 200 || !p.job.Preview || p.job.State != "simulated" {
		t.Fatal("preview ID escalated", code, p.job)
	}
	for _, outcome := range []string{"success", "partial", "cancelled"} {
		candidate := strings.ReplaceAll(body, "failed", outcome)
		if code := call(candidate); code != 202 {
			t.Fatal(outcome, code)
		}
		p.mu.Lock()
		p.finish("cancelled", "test")
		p.mu.Unlock()
	}
}
func TestPowerExactlyOnceAndPreview(t *testing.T) {
	for _, preview := range []bool{false, true} {
		p := powerFixture(t)
		now := time.Now()
		calls := 0
		p.shutdown = func() error { calls++; return nil }
		p.job = &powerJob{ID: "job", State: "countdown", created: now, heartbeat: now, Deadline: now.UnixMilli(), Preview: preview}
		p.tick(now)
		p.tick(now.Add(time.Second))
		want := 1
		if preview {
			want = 0
		}
		if calls != want {
			t.Fatal(preview, calls)
		}
		if !preview && p.begin() {
			t.Fatal("data admitted after shutdown")
		}
	}
	p := powerFixture(t)
	now := time.Now()
	p.job = &powerJob{ID: "job", State: "countdown", created: now, heartbeat: now, Deadline: now.UnixMilli()}
	p.shutdown = func() error { return errors.New("denied") }
	p.tick(now)
	if p.job.State != "error" {
		t.Fatal(p.job)
	}
}
func TestPowerCredentialsReplayAndRestart(t *testing.T) {
	p := powerFixture(t)
	s := &HTTPServer{power: p, listen: "127.0.0.1:8765"}
	body := `{"job_id":"unique-backup-job-123","result":"success"}`
	request := func(token, body string) int {
		r := httptest.NewRequest("POST", "/api/v1/power/complete", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.powerComplete(w, r)
		return w.Code
	}
	if n := request("DAV-password", body); n != 401 {
		t.Fatal(n)
	}
	if n := request("test-power-token", strings.Replace(body, "success", "failed", 1)); n != 400 {
		t.Fatal(n)
	}
	if n := request("test-power-token", body); n != 202 {
		t.Fatal(n)
	}
	original := p.job
	p.mu.Lock()
	p.finish("cancelled", "test")
	p.mu.Unlock()
	if n := request("test-power-token", body); n != 200 || p.job != original || p.job.State != "cancelled" {
		t.Fatal(n)
	}
	p.close()
	restarted := newPowerManager(p.store)
	t.Cleanup(restarted.close)
	s.power = restarted
	if restarted.job != nil {
		t.Fatal("restart resurrected shutdown")
	}
	if n := request("test-power-token", body); n != 200 || restarted.job != nil {
		t.Fatal(n)
	}
}
func TestPowerDesktopLocalOnlyAndHeartbeatReconnect(t *testing.T) {
	p := powerFixture(t)
	now := time.Now()
	p.job = &powerJob{ID: "job", State: "countdown", created: now, heartbeat: now.Add(-time.Minute), Deadline: now.Add(-time.Second).UnixMilli(), desktopHash: SHA256Bytes([]byte("desktop-secret"))}
	s := &HTTPServer{power: p}
	for _, remote := range []string{"192.168.1.4:1234", "127.0.0.1:1234"} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/v1/power/desktop", strings.NewReader(`{"action":"heartbeat"}`))
		r.RemoteAddr = remote
		r.Header.Set("Authorization", "Bearer desktop-secret")
		r.Header.Set("X-SB-Admin", "1")
		w := httptest.NewRecorder()
		s.powerDesktop(w, r)
		if strings.HasPrefix(remote, "192") {
			if w.Code != 403 {
				t.Fatal(w.Code)
			}
		} else {
			if w.Code != 200 || p.job.Deadline != 0 {
				t.Fatal(w.Code, p.job)
			}
		}
	}
}
