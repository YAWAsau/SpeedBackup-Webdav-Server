package sbserver

import (
	"encoding/json"
	"io"
	"net/http"
	"speedbackup-server/internal/service"
)

func (s *HTTPServer) adminAutostart(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth.validSession(r) {
		writeErr(w, 403, "administrator session required")
		return
	}
	serveAutostart(w, r, service.GetAutostart, service.UpdateAutostart)
}

func serveAutostart(w http.ResponseWriter, r *http.Request, get func() (service.AutostartState, error), set func(bool) (service.AutostartState, error)) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == "GET" {
		state, err := get()
		if err != nil {
			writeErr(w, 503, "cannot read startup state: "+err.Error())
			return
		}
		writeJSON(w, 200, state)
		return
	}
	if r.Method != "POST" {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256))
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil || req.Enabled == nil {
		writeErr(w, 400, "enabled boolean required")
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeErr(w, 400, "one JSON object required")
		return
	}
	state, err := set(*req.Enabled)
	if err != nil {
		writeErr(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, state)
}
