package sbserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLANAdministratorAndPickerOwner(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetAdministrator("admin", "x"); err != nil {
		t.Fatal(err)
	}
	s := &HTTPServer{store: store, adminAuth: newAdminAuth(store)}
	login := httptest.NewRequest("POST", "http://192.0.2.10:8765/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"x"}`))
	login.RemoteAddr = "192.0.2.20:1234"
	login.Header.Set("X-SB-Admin", "1")
	login.Header.Set("Origin", "http://192.0.2.10:8765")
	w := httptest.NewRecorder()
	s.adminAuth.credentials(w, login)
	if w.Code != 200 {
		t.Fatal("LAN login", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	r := httptest.NewRequest("GET", "http://192.0.2.10:8765/api/v1/admin/directories/picker", nil)
	r.RemoteAddr = "192.0.2.20:1234"
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.adminPicker(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"available":false`) {
		t.Fatal("LAN picker fallback", w.Code, w.Body.String())
	}
	id := strings.Repeat("a", 64)
	s.pickers.jobs = map[string]*pickerJob{id: {Owner: SHA256Bytes([]byte(cookie.Value)), Until: time.Now().Add(time.Minute), State: "selected", Path: store.Root}}
	local := func(c *http.Cookie) *http.Request {
		req := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/v1/admin/directories/picker/"+id, nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.SetPathValue("id", id)
		req.AddCookie(c)
		return req
	}
	// A different valid login cannot read the original selection.
	secondLogin := httptest.NewRequest("POST", "http://192.0.2.10:8765/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"x"}`))
	secondLogin.RemoteAddr = "192.0.2.21:2345"
	secondLogin.Header.Set("X-SB-Admin", "1")
	second := httptest.NewRecorder()
	s.adminAuth.credentials(second, secondLogin)
	if second.Code != 200 {
		t.Fatal(second.Code)
	}
	w = httptest.NewRecorder()
	s.adminPicker(w, local(second.Result().Cookies()[0]))
	if w.Code != 404 {
		t.Fatal("wrong owner read", w.Code)
	}
	s.pickers.jobs[id].Owner = SHA256Bytes([]byte(cookie.Value))
	w = httptest.NewRecorder()
	s.adminPicker(w, local(cookie))
	if w.Code != 200 || s.pickers.jobs[id] != nil {
		t.Fatal("result not consumed", w.Code)
	}
	w = httptest.NewRecorder()
	s.adminPicker(w, local(cookie))
	if w.Code != 404 {
		t.Fatal("result replay", w.Code)
	}
}
