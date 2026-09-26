package sbserver

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HTTPServer struct {
	store         *Store
	listen        string
	started       time.Time
	tokenMu       sync.RWMutex
	tokenHash     string
	mapsMu        sync.Mutex
	sessionGates  map[string]*sync.RWMutex
	sessionMeta   map[string]*sync.Mutex
	objectLocks   map[string]*sync.Mutex
	web           fs.FS
	dav           *davService
	adminAuth     *adminAuth
	pickers       pickerJobs
	debugExportMu sync.Mutex
}

func NewHTTPHandler(store *Store, listen, tokenHash string, webFS fs.FS) http.Handler {
	s := &HTTPServer{
		store:        store,
		listen:       listen,
		started:      time.Now(),
		tokenHash:    tokenHash,
		sessionGates: map[string]*sync.RWMutex{},
		sessionMeta:  map[string]*sync.Mutex{},
		objectLocks:  map[string]*sync.Mutex{},
		web:          webFS,
		dav:          newDAVService(store),
		adminAuth:    newAdminAuth(store),
	}
	return s.routes()
}

func (s *HTTPServer) getGate(id string) *sync.RWMutex {
	s.mapsMu.Lock()
	defer s.mapsMu.Unlock()
	if x := s.sessionGates[id]; x != nil {
		return x
	}
	x := &sync.RWMutex{}
	s.sessionGates[id] = x
	return x
}

func (s *HTTPServer) getMetaLock(id string) *sync.Mutex {
	s.mapsMu.Lock()
	defer s.mapsMu.Unlock()
	if x := s.sessionMeta[id]; x != nil {
		return x
	}
	x := &sync.Mutex{}
	s.sessionMeta[id] = x
	return x
}

func (s *HTTPServer) getObjectLock(key string) *sync.Mutex {
	s.mapsMu.Lock()
	defer s.mapsMu.Unlock()
	if x := s.objectLocks[key]; x != nil {
		return x
	}
	x := &sync.Mutex{}
	s.objectLocks[key] = x
	return x
}

func (s *HTTPServer) routes() http.Handler {
	public := http.NewServeMux()
	public.HandleFunc("GET /api/v1/capabilities", s.capabilities)
	public.HandleFunc("GET /api/v1/auth/status", s.adminAuth.status)
	public.HandleFunc("POST /api/v1/auth/setup", s.adminAuth.credentials)
	public.HandleFunc("POST /api/v1/auth/login", s.adminAuth.credentials)
	public.HandleFunc("POST /api/v1/auth/logout", s.adminAuth.logout)
	public.HandleFunc("GET /api/v1/native-picker/{id}", s.nativePicker)
	public.HandleFunc("POST /api/v1/native-picker/{id}", s.nativePicker)
	public.HandleFunc("GET /web/admin", s.webIndex)
	public.HandleFunc("GET /web/admin/", s.webIndex)
	public.HandleFunc("GET /web/admin/app.js", s.webAsset("web/app.js"))
	public.HandleFunc("GET /web/admin/dav_config.js", s.webAsset("web/dav_config.js"))
	public.HandleFunc("GET /web/admin/dav_monitor.js", s.webAsset("web/dav_monitor.js"))
	public.HandleFunc("GET /web/admin/preferences.js", s.webAsset("web/preferences.js"))
	public.HandleFunc("GET /web/admin/request.js", s.webAsset("web/request.js"))
	public.HandleFunc("GET /web/admin/dav_editor.js", s.webAsset("web/dav_editor.js"))
	public.HandleFunc("GET /web/admin/styles.css", s.webAsset("web/styles.css"))
	public.HandleFunc("GET /web/admin/oppo-sans-4-6c7d5864c661.ttf", s.webAsset("web/oppo-sans-4-6c7d5864c661.ttf"))
	public.HandleFunc("GET /web/admin/OPPO-Sans-LICENSE.txt", s.webAsset("web/OPPO-Sans-LICENSE.txt"))
	public.HandleFunc("GET /web/admin/server-mark.svg", s.webAsset("web/server-mark.svg"))
	public.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/web/admin", http.StatusTemporaryRedirect)
			return
		}
		http.NotFound(w, r)
	})

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/status", s.status)
	protected.HandleFunc("POST /api/v1/sessions", s.createSession)
	protected.HandleFunc("GET /api/v1/sessions/{id}", s.getSession)
	protected.HandleFunc("DELETE /api/v1/sessions/{id}", s.deleteSession)
	protected.HandleFunc("PUT /api/v1/sessions/{id}/objects/{sha}", s.putObject)
	protected.HandleFunc("PATCH /api/v1/sessions/{id}/objects/{sha}", s.patchObject)
	protected.HandleFunc("HEAD /api/v1/sessions/{id}/objects/{sha}", s.headObject)
	protected.HandleFunc("POST /api/v1/sessions/{id}/verify", s.verifySession)
	protected.HandleFunc("POST /api/v1/sessions/{id}/commit", s.commitSession)
	protected.HandleFunc("GET /api/v1/manifests/current", s.currentManifest)
	protected.HandleFunc("POST /api/v1/manifests/diff", s.manifestDiff)
	protected.HandleFunc("GET /api/v1/objects/{sha}", s.getObject)
	protected.HandleFunc("POST /api/v1/appdetails/audit", s.appdetailsAudit)
	protected.HandleFunc("GET /api/v1/admin/sessions", s.adminSessions)
	protected.HandleFunc("GET /api/v1/admin/profiles", s.adminProfiles)
	protected.HandleFunc("GET /api/v1/admin/storage", s.adminStorage)
	protected.HandleFunc("GET /api/v1/admin/events", s.adminEvents)
	protected.HandleFunc("GET /api/v1/admin/diagnostics", s.adminDiagnostics)
	protected.HandleFunc("GET /api/v1/admin/diagnostics/export", s.adminDiagnosticExport)
	protected.HandleFunc("POST /api/v1/admin/diagnostics/client-error", s.adminClientError)
	protected.HandleFunc("POST /api/v1/admin/cleanup", s.adminCleanup)
	protected.HandleFunc("POST /api/v1/admin/token/rotate", s.rotateToken)
	protected.HandleFunc("GET /api/v1/admin/webdav", s.dav.admin)
	protected.HandleFunc("POST /api/v1/admin/webdav", s.dav.admin)
	protected.HandleFunc("GET /api/v1/admin/webdav/activity", s.dav.activityHandler)
	protected.HandleFunc("GET /api/v1/admin/webdav/connection", s.webDAVConnection)
	protected.HandleFunc("GET /api/v1/admin/directories", s.dav.directories)
	protected.HandleFunc("POST /api/v1/admin/password", s.adminAuth.password)
	protected.HandleFunc("GET /api/v1/admin/autostart", s.adminAutostart)
	protected.HandleFunc("POST /api/v1/admin/autostart", s.adminAutostart)
	protected.HandleFunc("GET /api/v1/admin/directories/picker", s.adminPicker)
	protected.HandleFunc("POST /api/v1/admin/directories/picker", s.adminPicker)
	protected.HandleFunc("GET /api/v1/admin/directories/picker/{id}", s.adminPicker)
	protected.HandleFunc("DELETE /api/v1/admin/directories/picker/{id}", s.adminPicker)

	root := http.NewServeMux()
	root.Handle("/api/v1/capabilities", public)
	root.Handle("/api/v1/auth/", public)
	root.Handle("/api/v1/native-picker/", public)
	root.Handle("/web/", public)
	root.Handle("/api/", s.auth(protected))
	root.Handle("/", public)
	// Bypass ServeMux path cleaning for DAV. Its redirect on // would discard
	// some streaming client request bodies and is unnecessary for rooted paths.
	return securityHeaders(s.traceRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/web" || strings.HasPrefix(r.URL.Path, "/web/") {
			root.ServeHTTP(w, r)
			return
		}
		// Browsers can still open the management page from the origin. DAV
		// methods and file requests never redirect, including streaming PUT.
		if r.Method == "GET" && r.URL.Path == "/" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/web/admin", http.StatusTemporaryRedirect)
			return
		}
		s.dav.ServeHTTP(w, r)
	})))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *HTTPServer) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if s.adminAuth.validSession(r) {
			if r.Method != "GET" && r.Method != "HEAD" && !browserWrite(r) {
				writeErr(w, http.StatusForbidden, "same-origin admin request required")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		got := SHA256Bytes([]byte(strings.TrimPrefix(h, "Bearer ")))
		s.tokenMu.RLock()
		want := s.tokenHash
		s.tokenMu.RUnlock()
		if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": http.StatusText(status), "message": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		writeErr(w, http.StatusBadRequest, "multiple JSON values")
		return false
	}
	return true
}

func ptrEq(a, b *uint64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func profileDefault(v string) string {
	if v == "" {
		return "default"
	}
	return v
}

func (s *HTTPServer) event(name, message string, details any) {
	_ = s.store.AppendEvent(AuditEvent{Unix: nowUnix(), Event: name, Message: message, Details: details})
}

func (s *HTTPServer) capabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"server":   "SpeedBackup Server",
		"version":  ServerVersion,
		"protocol": ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor},
		"webdav":   map[string]any{"path": "/", "authentication": "basic", "progress_scope": "server_transport_only", "storage": "per_user_files"},
		"features": map[string]bool{
			"atomic_publish":            true,
			"content_addressed_objects": true,
			"resumable_upload":          true,
			"sha256_verify":             true,
			"manifest_diff":             true,
			"generation_guard":          true,
			"appdetails_audit":          true,
			"orphan_cleanup_safe":       true,
			"append_only_metadata":      true,
			"web_admin":                 true,
			"zh_tw_zh_cn":               true,
		},
	})
}

func (s *HTTPServer) status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"server":         "SpeedBackup Server",
		"version":        ServerVersion,
		"uptime_seconds": uint64(time.Since(s.started).Seconds()),
		"root":           s.store.Root,
		"listen":         s.listen,
		"protocol":       ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor},
	})
}

func (s *HTTPServer) createSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ProfileID = profileDefault(req.ProfileID)
	if err := ValidateIdentifier(req.DeviceID, "device_id"); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ValidateIdentifier(req.ProfileID, "profile_id"); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	cur, err := s.store.CurrentGenerationNumber(req.DeviceID, req.ProfileID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ptrEq(cur, req.BaseGeneration) {
		writeErr(w, http.StatusConflict, "base_generation mismatch")
		return
	}
	tok, err := GenerateToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id := "s_" + strings.TrimPrefix(tok, "sb1_")[:24]
	now := nowUnix()
	m := SessionMeta{
		Schema:         SessionSchema,
		ID:             id,
		DeviceID:       req.DeviceID,
		ProfileID:      req.ProfileID,
		BaseGeneration: req.BaseGeneration,
		State:          "open",
		CreatedUnix:    now,
		UpdatedUnix:    now,
		Uploads:        map[string]UploadedObject{},
	}
	if err := s.store.SaveSession(m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.event("session.create", "session created", map[string]any{"session_id": id, "device_id": req.DeviceID, "profile_id": req.ProfileID})
	writeJSON(w, http.StatusCreated, CreateSessionResponse{Session: m, CurrentGeneration: cur})
}

func (s *HTTPServer) getSession(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.LoadSession(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeErr(w, http.StatusNotFound, "session not found")
		} else {
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *HTTPServer) deleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	gate := s.getGate(id)
	gate.Lock()
	defer gate.Unlock()
	m, err := s.store.LoadSession(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	if m.State == "committed" {
		writeErr(w, http.StatusConflict, "committed session metadata is retained for audit")
		return
	}
	if err := os.RemoveAll(s.store.sessionDir(id)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.event("session.abort", "session deleted", map[string]any{"session_id": id})
	w.WriteHeader(http.StatusNoContent)
}

func parseUploadHeaders(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	off, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil || off < 0 {
		writeErr(w, http.StatusBadRequest, "invalid Upload-Offset")
		return 0, 0, false
	}
	total, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil || total < 0 {
		writeErr(w, http.StatusBadRequest, "invalid Upload-Length")
		return 0, 0, false
	}
	if off > total {
		writeErr(w, http.StatusBadRequest, "offset exceeds Upload-Length")
		return 0, 0, false
	}
	return off, total, true
}

func (s *HTTPServer) updateUploadContract(id, hash string, total int64) error {
	lock := s.getMetaLock(id)
	lock.Lock()
	defer lock.Unlock()
	m, err := s.store.LoadSession(id)
	if err != nil {
		return err
	}
	if m.State != "open" && m.State != "verified" {
		return fmt.Errorf("session is not writable")
	}
	if m.Uploads == nil {
		m.Uploads = map[string]UploadedObject{}
	}
	u, exists := m.Uploads[hash]
	if exists && u.ExpectedSize != total {
		return fmt.Errorf("Upload-Length changed for object")
	}
	u.SHA256 = hash
	u.ExpectedSize = total
	u.UpdatedUnix = nowUnix()
	m.Uploads[hash] = u
	m.UpdatedUnix = nowUnix()
	return s.store.SaveSession(m)
}

func (s *HTTPServer) finalizeUploadMeta(id, hash string, total, received int64, complete bool) error {
	lock := s.getMetaLock(id)
	lock.Lock()
	defer lock.Unlock()
	m, err := s.store.LoadSession(id)
	if err != nil {
		return err
	}
	if m.Uploads == nil {
		m.Uploads = map[string]UploadedObject{}
	}
	u := m.Uploads[hash]
	u.SHA256 = hash
	u.ExpectedSize = total
	u.ReceivedSize = received
	u.Complete = complete
	u.UpdatedUnix = nowUnix()
	m.Uploads[hash] = u
	m.State = "open"
	m.UpdatedUnix = nowUnix()
	return s.store.SaveSession(m)
}

func (s *HTTPServer) putObject(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upload-Offset") == "" {
		r.Header.Set("Upload-Offset", "0")
	}
	if r.Header.Get("Upload-Length") == "" && r.Header.Get("X-SpeedBackup-Size") != "" {
		r.Header.Set("Upload-Length", r.Header.Get("X-SpeedBackup-Size"))
	}
	if r.Header.Get("Upload-Length") == "" {
		if r.ContentLength < 0 {
			writeErr(w, http.StatusLengthRequired, "Content-Length or Upload-Length required")
			return
		}
		r.Header.Set("Upload-Length", strconv.FormatInt(r.ContentLength, 10))
	}
	s.patchObject(w, r)
}

func (s *HTTPServer) patchObject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	hash := strings.ToLower(r.PathValue("sha"))
	if ValidateIdentifier(id, "session_id") != nil || ValidateSHA256(hash) != nil {
		writeErr(w, http.StatusBadRequest, "invalid session or sha256")
		return
	}
	off, total, ok := parseUploadHeaders(w, r)
	if !ok {
		return
	}
	gate := s.getGate(id)
	gate.RLock()
	defer gate.RUnlock()
	objLock := s.getObjectLock(id + ":" + hash)
	objLock.Lock()
	defer objLock.Unlock()
	if err := s.updateUploadContract(id, hash, total); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err := os.MkdirAll(s.store.sessionObjectsDir(id), 0700); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	part := s.store.partPath(id, hash)
	ready := s.store.readyPath(id, hash)
	if st, err := os.Stat(ready); err == nil {
		if st.Size() == total {
			_ = s.finalizeUploadMeta(id, hash, total, total, true)
			writeJSON(w, http.StatusOK, map[string]any{"sha256": hash, "received_size": total, "expected_size": total, "complete": true})
			return
		}
		writeErr(w, http.StatusConflict, "ready object size mismatch")
		return
	}
	f, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if st.Size() != off {
		_ = f.Close()
		writeErr(w, http.StatusConflict, fmt.Sprintf("offset mismatch: server=%d client=%d", st.Size(), off))
		return
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		_ = f.Close()
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	remaining := total - off
	lr := &io.LimitedReader{R: r.Body, N: remaining + 1}
	n, err := io.Copy(f, lr)
	if err != nil {
		_ = f.Truncate(off)
		_ = f.Close()
		writeErr(w, http.StatusBadRequest, "upload body error: "+err.Error())
		return
	}
	if n > remaining {
		_ = f.Truncate(off)
		_ = f.Close()
		writeErr(w, http.StatusRequestEntityTooLarge, "upload exceeds Upload-Length")
		return
	}
	received := off + n
	if err := f.Sync(); err != nil {
		_ = f.Close()
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	complete := received == total
	if complete {
		if err := f.Close(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		got, err := fileSHA256(part)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if got != hash {
			_ = os.Remove(part)
			_ = s.finalizeUploadMeta(id, hash, total, 0, false)
			writeErr(w, http.StatusConflict, "sha256 mismatch")
			return
		}
		if err := os.Rename(part, ready); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		_ = f.Close()
	}
	if err := s.finalizeUploadMeta(id, hash, total, received, complete); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sha256": hash, "received_size": received, "expected_size": total, "complete": complete})
}

func (s *HTTPServer) headObject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	hash := strings.ToLower(r.PathValue("sha"))
	m, err := s.store.LoadSession(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	u, ok := m.Uploads[hash]
	if !ok {
		writeErr(w, http.StatusNotFound, "upload not found")
		return
	}
	var offset int64
	if st, err := os.Stat(s.store.readyPath(id, hash)); err == nil {
		offset = st.Size()
	} else if st, err := os.Stat(s.store.partPath(id, hash)); err == nil {
		offset = st.Size()
	}
	w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
	w.Header().Set("Upload-Length", strconv.FormatInt(u.ExpectedSize, 10))
	if u.Complete {
		w.Header().Set("Upload-Complete", "1")
	} else {
		w.Header().Set("Upload-Complete", "0")
	}
	w.Header().Set("X-SpeedBackup-Complete", strconv.FormatBool(u.Complete))
	w.WriteHeader(http.StatusOK)
}

func (s *HTTPServer) verifySession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	gate := s.getGate(id)
	gate.Lock()
	defer gate.Unlock()
	lock := s.getMetaLock(id)
	lock.Lock()
	defer lock.Unlock()
	m, err := s.store.LoadSession(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	if len(m.Uploads) == 0 {
		writeErr(w, http.StatusConflict, "session has no uploads")
		return
	}
	for hash, u := range m.Uploads {
		if !u.Complete || u.ReceivedSize != u.ExpectedSize {
			writeErr(w, http.StatusConflict, "incomplete upload: "+hash)
			return
		}
		if ok, err := s.store.ObjectExistsWithSize(hash, u.ExpectedSize); err == nil && ok {
			continue
		}
		ready := s.store.readyPath(id, hash)
		st, err := os.Stat(ready)
		if err != nil || st.Size() != u.ExpectedSize {
			writeErr(w, http.StatusConflict, "missing verified upload: "+hash)
			return
		}
		got, err := fileSHA256(ready)
		if err != nil || got != hash {
			writeErr(w, http.StatusConflict, "sha256 verification failed: "+hash)
			return
		}
	}
	m.State = "verified"
	m.UpdatedUnix = nowUnix()
	if err := s.store.SaveSession(m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.event("session.verify", "session verified", map[string]any{"session_id": id})
	writeJSON(w, http.StatusOK, map[string]any{"verified": true, "session_id": id, "objects": len(m.Uploads)})
}

func (s *HTTPServer) commitSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req CommitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := ValidateIdentifier(req.CommitID, "commit_id"); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	gate := s.getGate(id)
	gate.Lock()
	defer gate.Unlock()
	s.store.commitMu.Lock()
	defer s.store.commitMu.Unlock()

	m, err := s.store.LoadSession(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	entries, err := NormalizeEntries(req.Entries)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	current, err := s.store.LoadCurrentGeneration(m.DeviceID, m.ProfileID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Crash-safe idempotency: a generation may have been published before
	// the final session metadata version was persisted.
	if current != nil && current.SessionID == id && current.CommitID == req.CommitID {
		g := current.Generation
		m.State = "committed"
		m.UpdatedUnix = nowUnix()
		m.CommittedGeneration = &g
		m.CommitID = req.CommitID
		_ = s.store.SaveSession(m)
		writeJSON(w, http.StatusOK, CommitResponse{Committed: true, IdempotentReplay: true, Generation: g, ManifestSHA256: current.ManifestSHA256})
		return
	}

	if m.State == "committed" {
		if m.CommitID == req.CommitID && m.CommittedGeneration != nil {
			cm, err := s.store.LoadGeneration(m.DeviceID, m.ProfileID, *m.CommittedGeneration)
			if err == nil {
				writeJSON(w, http.StatusOK, CommitResponse{Committed: true, IdempotentReplay: true, Generation: *m.CommittedGeneration, ManifestSHA256: cm.ManifestSHA256})
				return
			}
		}
		writeErr(w, http.StatusConflict, "session already committed with different commit_id")
		return
	}

	var currentNum *uint64
	if current != nil {
		g := current.Generation
		currentNum = &g
	}
	if !ptrEq(currentNum, req.BaseGeneration) {
		writeErr(w, http.StatusConflict, "base_generation mismatch")
		return
	}
	if !ptrEq(m.BaseGeneration, req.BaseGeneration) {
		writeErr(w, http.StatusConflict, "session base_generation mismatch")
		return
	}

	for _, e := range entries {
		ok, err := s.store.ObjectExistsWithSize(e.SHA256, e.Size)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if ok {
			continue
		}
		st, err := os.Stat(s.store.readyPath(id, e.SHA256))
		if err != nil || st.Size() != e.Size {
			writeErr(w, http.StatusConflict, "manifest references missing object "+e.SHA256)
			return
		}
	}
	for _, e := range entries {
		if err := s.store.PromoteObject(id, e.SHA256, e.Size); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	generation := uint64(1)
	if currentNum != nil {
		generation = *currentNum + 1
	}
	manifest, err := BuildManifest(generation, m.DeviceID, m.ProfileID, id, req.CommitID, req.BaseGeneration, entries)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.PublishManifest(manifest); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	m.State = "committed"
	m.UpdatedUnix = nowUnix()
	m.CommittedGeneration = &generation
	m.CommitID = req.CommitID
	if err := s.store.SaveSession(m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.event("session.commit", "generation committed", map[string]any{"session_id": id, "generation": generation, "manifest_sha256": manifest.ManifestSHA256})
	writeJSON(w, http.StatusOK, CommitResponse{Committed: true, IdempotentReplay: false, Generation: generation, ManifestSHA256: manifest.ManifestSHA256})
}

func (s *HTTPServer) currentManifest(w http.ResponseWriter, r *http.Request) {
	device := r.URL.Query().Get("device_id")
	profile := profileDefault(r.URL.Query().Get("profile_id"))
	if ValidateIdentifier(device, "device_id") != nil || ValidateIdentifier(profile, "profile_id") != nil {
		writeErr(w, http.StatusBadRequest, "invalid device/profile")
		return
	}
	m, err := s.store.LoadCurrentGeneration(device, profile)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeErr(w, http.StatusNotFound, "no committed generation")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *HTTPServer) manifestDiff(w http.ResponseWriter, r *http.Request) {
	var req ManifestDiffRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ProfileID = profileDefault(req.ProfileID)
	if ValidateIdentifier(req.DeviceID, "device_id") != nil || ValidateIdentifier(req.ProfileID, "profile_id") != nil {
		writeErr(w, http.StatusBadRequest, "invalid device/profile")
		return
	}
	entries, err := NormalizeEntries(req.Entries)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	current, err := s.store.LoadCurrentGeneration(req.DeviceID, req.ProfileID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	remoteMap := map[string]ManifestEntry{}
	var base *uint64
	if current != nil {
		g := current.Generation
		base = &g
		for _, e := range current.Entries {
			remoteMap[e.Path] = e
		}
	}
	clientPaths := map[string]struct{}{}
	uploadSet := map[string]struct{}{}
	unchanged := []string{}
	changed := []string{}
	for _, e := range entries {
		clientPaths[e.Path] = struct{}{}
		if old, ok := remoteMap[e.Path]; ok && old.SHA256 == e.SHA256 && old.Size == e.Size {
			unchanged = append(unchanged, e.Path)
		} else {
			changed = append(changed, e.Path)
		}
		ok, err := s.store.ObjectExistsWithSize(e.SHA256, e.Size)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			uploadSet[e.SHA256] = struct{}{}
		}
	}
	remoteOnly := []string{}
	for p := range remoteMap {
		if _, ok := clientPaths[p]; !ok {
			remoteOnly = append(remoteOnly, p)
		}
	}
	uploads := make([]string, 0, len(uploadSet))
	for h := range uploadSet {
		uploads = append(uploads, h)
	}
	sort.Strings(uploads)
	sort.Strings(unchanged)
	sort.Strings(changed)
	sort.Strings(remoteOnly)
	writeJSON(w, http.StatusOK, ManifestDiffResponse{BaseGeneration: base, UploadRequiredSHA256: uploads, UnchangedPaths: unchanged, ChangedPaths: changed, RemoteOnlyPaths: remoteOnly})
}

func (s *HTTPServer) getObject(w http.ResponseWriter, r *http.Request) {
	hash := strings.ToLower(r.PathValue("sha"))
	if ValidateSHA256(hash) != nil {
		writeErr(w, http.StatusBadRequest, "invalid sha256")
		return
	}
	p := s.store.objectPath(hash)
	f, err := os.Open(p)
	if err != nil {
		writeErr(w, http.StatusNotFound, "object not found")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	w.Header().Set("X-SpeedBackup-SHA256", hash)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func (s *HTTPServer) appdetailsAudit(w http.ResponseWriter, r *http.Request) {
	var req AppDetailsAuditRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	writeJSON(w, http.StatusOK, AuditAppDetails(req))
}

func (s *HTTPServer) adminSessions(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.ListSessions()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *HTTPServer) adminProfiles(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.ListProfiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *HTTPServer) adminStorage(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.StorageStats()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *HTTPServer) adminEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 200
	}
	v, err := s.store.TailEvents(limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *HTTPServer) adminCleanup(w http.ResponseWriter, r *http.Request) {
	var req CleanupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	dry := true
	if req.DryRun != nil {
		dry = *req.DryRun
	}
	if !dry && req.Confirm != "DELETE_ORPHAN_OBJECTS" {
		writeErr(w, http.StatusBadRequest, "destructive cleanup requires confirm=DELETE_ORPHAN_OBJECTS")
		return
	}
	v, err := s.store.CleanupObjects(dry)
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	name := "cleanup.plan"
	if !dry {
		name = "cleanup.execute"
	}
	s.event(name, "object cleanup completed", v)
	writeJSON(w, http.StatusOK, v)
}

func (s *HTTPServer) rotateToken(w http.ResponseWriter, r *http.Request) {
	token, err := GenerateToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	hash := SHA256Bytes([]byte(token))
	cfg := ServerConfig{Schema: "speedbackup.server.config.v1", TokenSHA256: hash, CreatedUnix: nowUnix()}
	if err := s.store.SaveConfig(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.tokenMu.Lock()
	s.tokenHash = hash
	s.tokenMu.Unlock()
	s.event("auth.token.rotate", "server token rotated", map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "warning": "This token is shown once. Store it securely."})
}

func (s *HTTPServer) webIndex(w http.ResponseWriter, r *http.Request) {
	s.serveWebFile(w, "web/index.html")
}

func (s *HTTPServer) webAsset(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { s.serveWebFile(w, name) }
}

func (s *HTTPServer) serveWebFile(w http.ResponseWriter, name string) {
	var data []byte
	var err error
	if s.web != nil {
		data, err = fs.ReadFile(s.web, name)
	} else {
		data, err = embeddedWeb.ReadFile(name)
	}
	if err != nil {
		writeErr(w, http.StatusNotFound, "web asset not found")
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if name == "web/oppo-sans-4-6c7d5864c661.ttf" {
		w.Header().Set("Content-Type", "font/ttf")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
