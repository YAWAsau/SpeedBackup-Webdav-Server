package sbserver

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const adminCookie = "sb_admin_session"

type administrator struct {
	Username string `json:"username"`
	Salt     string `json:"salt"`
	Hash     string `json:"password_hash"`
}
type adminSession struct {
	Version string
	Until   time.Time
}
type loginWindow struct {
	Count int
	Until time.Time
}
type adminAuth struct {
	store    *Store
	mu       sync.Mutex
	sessions map[string]adminSession
	attempts map[string]loginWindow
}

func newAdminAuth(s *Store) *adminAuth {
	return &adminAuth{store: s, sessions: map[string]adminSession{}, attempts: map[string]loginWindow{}}
}
func (s *Store) loadAdministrator() (administrator, string, error) {
	var a administrator
	p, e := latestVersionFile(s.internal("administrators"))
	if e != nil {
		return a, "", e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return a, "", e
	}
	e = json.Unmarshal(b, &a)
	if e == nil && (!davUsername.MatchString(a.Username) || len(a.Hash) != 64 || len(a.Salt) != 32) {
		e = fmt.Errorf("invalid administrator configuration")
	}
	return a, p, e
}

// Caller must hold the server root lock, or use the running HTTP server.
func (s *Store) SetAdministrator(username, password string) error {
	if !davUsername.MatchString(username) {
		return fmt.Errorf("username: 1-48 lowercase letters, digits, _ or -")
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("password must not be blank")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return e
	}
	hash, e := pbkdf2.Key(sha256.New, password, salt, 210000, 32)
	if e != nil {
		return e
	}
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	dir := s.internal("administrators")
	n, e := nextVersion(dir)
	if e != nil {
		return e
	}
	return writeImmutableJSON(filepath.Join(dir, fmt.Sprintf("%020d.json", n)), administrator{username, hex.EncodeToString(salt), hex.EncodeToString(hash)})
}
func checkAdminPassword(a administrator, password string) bool {
	salt, e := hex.DecodeString(a.Salt)
	if e != nil {
		return false
	}
	want, e := hex.DecodeString(a.Hash)
	if e != nil {
		return false
	}
	got, e := pbkdf2.Key(sha256.New, password, salt, 210000, 32)
	return e == nil && subtle.ConstantTimeCompare(want, got) == 1
}

// Do not trust X-Forwarded-For for bootstrap authorization. A local browser or
// SSH localhost tunnel can establish the first administrator without a token.
func localBootstrap(r *http.Request) bool {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return false
	}
	ip := net.ParseIP(host)
	target := r.Host
	if h, _, e := net.SplitHostPort(target); e == nil {
		target = h
	}
	target = strings.Trim(target, "[]")
	targetIP := net.ParseIP(target)
	return ip != nil && ip.IsLoopback() && (target == "localhost" || (targetIP != nil && targetIP.IsLoopback())) && r.Header.Get("Forwarded") == "" && r.Header.Get("X-Forwarded-For") == ""
}
func browserWrite(r *http.Request) bool {
	if r.Header.Get("X-SB-Admin") != "1" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		return e == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host)
	}
	return true
}
func (a *adminAuth) validSession(r *http.Request) bool {
	c, e := r.Cookie(adminCookie)
	if e != nil || len(c.Value) != 64 {
		return false
	}
	key := SHA256Bytes([]byte(c.Value))
	a.mu.Lock()
	defer a.mu.Unlock()
	v, ok := a.sessions[key]
	if !ok {
		return false
	}
	_, version, e := a.store.loadAdministrator()
	if e != nil || version != v.Version || time.Now().After(v.Until) {
		delete(a.sessions, key)
		return false
	}
	return true
}
func (a *adminAuth) issue(w http.ResponseWriter, r *http.Request) error {
	token := make([]byte, 32)
	if _, e := rand.Read(token); e != nil {
		return e
	}
	_, version, e := a.store.loadAdministrator()
	if e != nil {
		return e
	}
	now := time.Now()
	for k, v := range a.sessions {
		if now.After(v.Until) {
			delete(a.sessions, k)
		}
	}
	if len(a.sessions) >= 128 {
		return fmt.Errorf("too many active admin sessions")
	}
	value := hex.EncodeToString(token)
	a.sessions[SHA256Bytes([]byte(value))] = adminSession{version, now.Add(8 * time.Hour)}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: value, Path: "/api/", HttpOnly: true, Secure: r.TLS != nil || strings.HasPrefix(r.Header.Get("Origin"), "https://"), SameSite: http.SameSiteStrictMode, MaxAge: 8 * 3600})
	return nil
}
func (a *adminAuth) status(w http.ResponseWriter, r *http.Request) {
	account, _, e := a.store.loadAdministrator()
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		writeErr(w, 500, "cannot read administrator configuration")
		return
	}
	signed := a.validSession(r)
	name := ""
	if signed {
		name = account.Username
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"setup_required": errors.Is(e, os.ErrNotExist), "local_setup_allowed": localBootstrap(r), "authenticated": signed, "username": name})
}
func (a *adminAuth) credentials(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !browserWrite(r) {
		writeErr(w, 403, "same-origin admin request required")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeErr(w, 400, "invalid credentials request")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	setup := strings.HasSuffix(r.URL.Path, "/setup")
	account, _, e := a.store.loadAdministrator()
	if setup {
		if !errors.Is(e, os.ErrNotExist) {
			if e != nil {
				writeErr(w, 500, "cannot read administrator configuration")
			} else {
				writeErr(w, 409, "administrator already exists")
			}
			return
		}
		if !localBootstrap(r) {
			writeErr(w, 403, "create the first administrator using localhost or an SSH localhost tunnel")
			return
		}
		if e = a.store.SetAdministrator(req.Username, req.Password); e != nil {
			writeErr(w, 400, e.Error())
			return
		}
	} else {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		now := time.Now()
		for k, v := range a.attempts {
			if now.After(v.Until) {
				delete(a.attempts, k)
			}
		}
		v := a.attempts[host]
		if v.Until.IsZero() {
			v.Until = now.Add(time.Minute)
		}
		if v.Count >= 12 || len(a.attempts) >= 1024 {
			w.Header().Set("Retry-After", "60")
			writeErr(w, 429, "too many login attempts; retry in one minute")
			return
		}
		v.Count++
		a.attempts[host] = v
		if e != nil || !checkAdminPassword(account, req.Password) || account.Username != req.Username {
			writeErr(w, 401, "invalid username or password")
			return
		}
		delete(a.attempts, host)
	}
	if e = a.issue(w, r); e != nil {
		writeErr(w, 503, "cannot establish admin session")
		return
	}
	writeJSON(w, 200, map[string]any{"username": req.Username})
}
func (a *adminAuth) logout(w http.ResponseWriter, r *http.Request) {
	if !browserWrite(r) {
		writeErr(w, 403, "same-origin admin request required")
		return
	}
	a.mu.Lock()
	if c, e := r.Cookie(adminCookie); e == nil {
		delete(a.sessions, SHA256Bytes([]byte(c.Value)))
	}
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Path: "/api/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]bool{"logged_out": true})
}
func (a *adminAuth) password(w http.ResponseWriter, r *http.Request) {
	if !browserWrite(r) || !a.validSession(r) {
		writeErr(w, 403, "admin session required")
		return
	}
	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeErr(w, 400, "invalid request")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	account, _, e := a.store.loadAdministrator()
	if e != nil || !checkAdminPassword(account, req.Current) {
		writeErr(w, 401, "current password is incorrect")
		return
	}
	if e = a.store.SetAdministrator(account.Username, req.New); e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	a.sessions = map[string]adminSession{}
	writeJSON(w, 200, map[string]bool{"login_required": true})
}
