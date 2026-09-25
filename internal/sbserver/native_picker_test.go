package sbserver

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPickerCapability(t *testing.T) {
	id := strings.Repeat("a", 64)
	path := t.TempDir()
	s := &HTTPServer{pickers: pickerJobs{jobs: map[string]*pickerJob{id: {Until: time.Now().Add(time.Minute), State: "waiting", Directory: path}}}}
	invoke := func(method, remote, host, body, origin string) int {
		r := httptest.NewRequest(method, "http://127.0.0.1:8765/api/v1/native-picker/"+id, strings.NewReader(body))
		r.RemoteAddr = remote
		r.Host = host
		r.SetPathValue("id", id)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		s.nativePicker(w, r)
		return w.Code
	}
	if invoke("GET", "192.0.2.2:99", "127.0.0.1:8765", "", "") != 403 {
		t.Fatal("remote allowed")
	}
	if invoke("GET", "127.0.0.1:99", "evil.test:8765", "", "") != 403 {
		t.Fatal("rebind allowed")
	}
	if invoke("GET", "127.0.0.1:99", "127.0.0.1:8765", "", "http://127.0.0.1:8765") != 403 {
		t.Fatal("browser claim allowed")
	}
	if invoke("POST", "127.0.0.1:99", "127.0.0.1:8765", `{"cancelled":true}`, "") != 409 {
		t.Fatal("unclaimed completion")
	}
	if invoke("GET", "127.0.0.1:99", "127.0.0.1:8765", "", "") != 200 {
		t.Fatal("claim failed")
	}
	if invoke("GET", "127.0.0.1:99", "127.0.0.1:8765", "", "") != 409 {
		t.Fatal("double claim")
	}
	if invoke("POST", "127.0.0.1:99", "127.0.0.1:8765", `{"path":"relative"}`, "") != 400 {
		t.Fatal("relative accepted")
	}
	b, _ := json.Marshal(map[string]string{"path": path})
	if invoke("POST", "127.0.0.1:99", "127.0.0.1:8765", string(b), "") != 200 || s.pickers.jobs[id].Path != path {
		t.Fatal("result failed")
	}
	if invoke("POST", "127.0.0.1:99", "127.0.0.1:8765", `{"cancelled":true}`, "") != 409 {
		t.Fatal("result replay")
	}
	s.pickers.jobs[id].Until = time.Now().Add(-time.Second)
	if invoke("GET", "127.0.0.1:99", "127.0.0.1:8765", "", "") != 404 {
		t.Fatal("expired request")
	}
}
