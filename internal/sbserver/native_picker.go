package sbserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"speedbackup-server/internal/desktop"
	"strconv"
	"sync"
	"time"
)

type pickerJob struct {
	Owner     string
	Until     time.Time
	Directory string
	State     string
	Path      string
	Error     string
}
type pickerJobs struct {
	mu   sync.Mutex
	jobs map[string]*pickerJob
}

func (p *pickerJobs) cleanup() {
	for id, j := range p.jobs {
		if time.Now().After(j.Until) {
			delete(p.jobs, id)
		}
	}
}
func pickerLocal(r *http.Request) bool {
	// HTTP callback is always direct loopback; forwarded origins are not accepted.
	return localBootstrap(r) && r.TLS == nil && r.Header.Get("X-Forwarded-For") == "" && r.Header.Get("Forwarded") == ""
}
func pickerPort(r *http.Request) string {
	_, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		return ""
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return ""
	}
	return port
}
func (s *HTTPServer) adminPicker(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth.validSession(r) {
		writeErr(w, 401, "administrator login required")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	available := pickerLocal(r) && pickerPort(r) != "" && desktop.Available()
	if r.Method == "GET" && r.PathValue("id") == "" {
		writeJSON(w, 200, map[string]bool{"available": available})
		return
	}
	if !pickerLocal(r) {
		writeErr(w, 403, "native picker requires the server's local browser")
		return
	}
	cookie, _ := r.Cookie(adminCookie)
	owner := SHA256Bytes([]byte(cookie.Value))
	s.pickers.mu.Lock()
	defer s.pickers.mu.Unlock()
	s.pickers.cleanup()
	if r.Method == "POST" {
		if !available {
			writeErr(w, 409, "native picker unavailable; use server directory browser")
			return
		}
		var input struct {
			Directory string `json:"directory"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if !decodePickerJSON(r, &input) {
			writeErr(w, 400, "invalid picker request")
			return
		}
		if len(s.pickers.jobs) >= 32 {
			writeErr(w, 429, "too many folder picker requests")
			return
		}
		idBytes := make([]byte, 32)
		if _, err := rand.Read(idBytes); err != nil {
			writeErr(w, 500, "unable to create picker request")
			return
		}
		id := hex.EncodeToString(idBytes)
		if s.pickers.jobs == nil {
			s.pickers.jobs = map[string]*pickerJob{}
		}
		s.pickers.jobs[id] = &pickerJob{Owner: owner, Until: time.Now().Add(3 * time.Minute), Directory: input.Directory, State: "waiting"}
		writeJSON(w, 200, map[string]string{"id": id, "launch_url": desktop.Protocol + "://choose/?port=" + pickerPort(r) + "&request=" + id})
		return
	}
	id := r.PathValue("id")
	job := s.pickers.jobs[id]
	if job == nil || job.Owner != owner {
		writeErr(w, 404, "folder picker expired or not found")
		return
	}
	if r.Method == "DELETE" {
		delete(s.pickers.jobs, id)
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	writeJSON(w, 200, map[string]string{"state": job.State, "path": job.Path, "error": job.Error})
	if job.State != "waiting" && job.State != "choosing" {
		delete(s.pickers.jobs, id)
	}
}

func decodePickerJSON(r *http.Request, out any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}

func (s *HTTPServer) nativePicker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// A browser cannot claim or complete the capability. The helper has no Origin
	// or Fetch Metadata headers. Paths are returned only to the creating session.
	if !pickerLocal(r) || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
		writeErr(w, 403, "local desktop helper required")
		return
	}
	s.pickers.mu.Lock()
	defer s.pickers.mu.Unlock()
	s.pickers.cleanup()
	job := s.pickers.jobs[r.PathValue("id")]
	if job == nil {
		writeErr(w, 404, "folder picker expired or not found")
		return
	}
	if r.Method == "GET" {
		if job.State != "waiting" {
			writeErr(w, 409, "folder picker already claimed")
			return
		}
		job.State = "choosing"
		writeJSON(w, 200, map[string]string{"directory": job.Directory})
		return
	}
	if job.State != "choosing" {
		writeErr(w, 409, "folder picker not awaiting result")
		return
	}
	var input struct {
		Path      string `json:"path"`
		Cancelled bool   `json:"cancelled"`
		Error     string `json:"error"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	if !decodePickerJSON(r, &input) {
		writeErr(w, 400, "invalid picker result")
		return
	}
	if input.Cancelled {
		job.State = "cancelled"
	} else if input.Error != "" {
		job.State = "error"
		job.Error = "Windows 無法開啟資料夾選擇視窗，請改用網頁瀏覽。"
	} else {
		if !filepath.IsAbs(input.Path) {
			writeErr(w, 400, "absolute directory required")
			return
		}
		st, err := os.Stat(input.Path)
		if err != nil || !st.IsDir() {
			writeErr(w, 400, "directory does not exist or is inaccessible to service")
			return
		}
		job.State = "selected"
		job.Path = filepath.Clean(input.Path)
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
