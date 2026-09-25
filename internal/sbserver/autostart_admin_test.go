package sbserver

import (
	"errors"
	"net/http/httptest"
	"speedbackup-server/internal/service"
	"strings"
	"testing"
)

func TestAutostartAPI(t *testing.T) {
	enabled := true
	state := service.AutostartState{Supported: true, Installed: true, Manageable: true, Enabled: &enabled}
	get := func() (service.AutostartState, error) { return state, nil }
	calls := 0
	set := func(value bool) (service.AutostartState, error) { calls++; enabled = value; return state, nil }
	for _, raw := range []string{`{}`, `null`, `{"enabled":null}`, `{"enabled":"true"}`, `{"enabled":true,"command":"stop"}`, `{"enabled":true} {}`, strings.Repeat(" ", 300)} {
		w := httptest.NewRecorder()
		serveAutostart(w, httptest.NewRequest("POST", "/", strings.NewReader(raw)), get, set)
		if w.Code != 400 || calls != 0 {
			t.Fatal(raw, w.Code, calls)
		}
	}
	w := httptest.NewRecorder()
	serveAutostart(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"enabled":false}`)), get, set)
	if w.Code != 200 || enabled || calls != 1 {
		t.Fatal(w.Code, enabled, calls)
	}
	w = httptest.NewRecorder()
	serveAutostart(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"enabled":true}`)), get, func(bool) (service.AutostartState, error) { return state, errors.New("permission denied") })
	if w.Code != 409 || enabled {
		t.Fatal(w.Code, enabled)
	}
	w = httptest.NewRecorder()
	serveAutostart(w, httptest.NewRequest("GET", "/", nil), func() (service.AutostartState, error) { return state, errors.New("unavailable") }, set)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
