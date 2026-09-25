package sbserver

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasswordsWithoutCharacterLengthPolicy(t *testing.T) {
	for _, tc := range []struct{ name, password string }{
		{"one_character", "x"},
		{"unicode", "密"},
		{"over_previous_limits", strings.Repeat("密", 800)},
		{"preserve_spaces", " x "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, e := NewStore(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { _ = store.diagnostics.flush() })
			s := httptest.NewServer(NewHTTPHandler(store, "local", SHA256Bytes([]byte("legacy")), embeddedWeb))
			defer s.Close()
			jar, _ := cookiejar.New(nil)
			client := &http.Client{Jar: jar}
			credentials := map[string]string{"username": "admin", "password": tc.password}
			expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/auth/setup", credentials, nil), 200)
			expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/auth/login", credentials, nil), 200)
			account := map[string]string{"username": "phone", "password": tc.password}
			expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/admin/webdav", account, nil), 200)
			checkDAV := func(password string, code int) {
				r, _ := http.NewRequest("PROPFIND", s.URL+"/dav/", nil)
				r.SetBasicAuth("phone", password)
				r.Header.Set("Depth", "0")
				resp, err := client.Do(r)
				if err != nil {
					t.Fatal(err)
				}
				expectStatus(t, resp, code)
			}
			checkDAV(tc.password, 207)
			account["password"] = ""
			expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/admin/webdav", account, nil), 200)
			checkDAV(tc.password, 207)
			for _, blank := range []string{"", " \t　"} {
				expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/admin/webdav", map[string]string{"username": "new", "password": blank}, nil), 400)
				expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/admin/password", map[string]string{"current_password": tc.password, "new_password": blank}, nil), 400)
				if err := store.SetAdministrator("admin", blank); err == nil {
					t.Fatal("blank administrator password accepted")
				}
			}
			account["password"] = " \t　"
			expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/admin/webdav", account, nil), 400)
			checkDAV(tc.password, 207)
			account["password"] = "z"
			expectStatus(t, adminRequest(t, client, "POST", s.URL+"/api/v1/admin/webdav", account, nil), 200)
			checkDAV(tc.password, 401)
			checkDAV("z", 207)
		})
	}
}
