package sbserver

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

func (s *HTTPServer) diagnosticInfo() map[string]any {
	return map[string]any{"version": ServerVersion, "go_version": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "pid": os.Getpid(), "created_at": time.Now().Format(time.RFC3339Nano), "started_at": s.started.Format(time.RFC3339Nano), "uptime_seconds": int64(time.Since(s.started).Seconds()), "listen": s.listen, "data_root": s.store.Root, "log_directory": s.store.internal("logs"), "audit_directory": s.store.internal("audit"), "logging": s.store.diagnostics.status(), "goroutines": runtime.NumGoroutine()}
}

func (s *HTTPServer) adminDiagnostics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.diagnosticInfo())
}
func (s *HTTPServer) adminDiagnosticExport(w http.ResponseWriter, r *http.Request) {
	if !s.debugExportMu.TryLock() {
		writeErr(w, 429, "another debug export is running")
		return
	}
	defer s.debugExportMu.Unlock()
	flushErr := s.store.diagnostics.flush()
	info := s.diagnosticInfo()
	if flushErr != nil {
		info["flush_error"] = flushErr.Error()
	}
	rows, revision, totals := s.dav.activity.snapshotState()
	info["activity"] = map[string]any{"transfers": rows, "revision": revision, "summary": totals, "scope": "server_transport_only"}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="speedbackup_server_debug_%s.zip"`, time.Now().Format("20060102-150405")))
	w.Header().Set("Cache-Control", "no-store")
	if err := WriteDebugBundle(s.store.Root, w, info); err != nil {
		requestTraceError(r, err)
	}
}

// Only named log files are read. Config, tokens, environment, process arguments,
// account hashes, browser storage and backup data are never traversed.
func WriteDebugBundle(root string, out io.Writer, info map[string]any) error {
	base, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer base.Close()
	z := zip.NewWriter(out)
	add := func(name string, data []byte) error {
		f, e := z.Create(name)
		if e != nil {
			return e
		}
		_, e = f.Write(data)
		return e
	}
	inventory := []map[string]any{}
	for _, group := range []struct{ dir, file string }{{"logs", "server.jsonl"}, {"audit", "events.jsonl"}} {
		for i := diagnosticCopies - 1; i >= 0; i-- {
			name := group.file
			if i > 0 {
				name += fmt.Sprintf(".%d", i)
			}
			rel := filepath.Join(".speedbackup-server", group.dir, name)
			// Reject symlink substitutions even when they point inside this root.
			st, e := base.Lstat(rel)
			if os.IsNotExist(e) {
				continue
			}
			entry := map[string]any{"name": group.dir + "/" + name}
			if e != nil {
				entry["error"] = e.Error()
				inventory = append(inventory, entry)
				continue
			}
			if !st.Mode().IsRegular() {
				entry["error"] = "not a regular log file"
				inventory = append(inventory, entry)
				continue
			}
			f, e := base.Open(rel)
			if e != nil {
				entry["error"] = e.Error()
				inventory = append(inventory, entry)
				continue
			}
			size := st.Size()
			start := max(int64(0), size-diagnosticFileLimit)
			data, e := io.ReadAll(io.NewSectionReader(f, start, size-start))
			_ = f.Close()
			if e != nil {
				entry["error"] = e.Error()
				inventory = append(inventory, entry)
				continue
			}
			if start > 0 {
				if p := bytes.IndexByte(data, '\n'); p >= 0 {
					data = data[p+1:]
				} else {
					data = nil
				}
			}
			// A concurrently-written final line is excluded and explicitly reported.
			partial := len(data) > 0 && data[len(data)-1] != '\n'
			if partial {
				if p := bytes.LastIndexByte(data, '\n'); p >= 0 {
					data = data[:p+1]
				} else {
					data = nil
				}
			}
			entry["source_bytes"] = size
			entry["exported_bytes"] = len(data)
			entry["truncated"] = start > 0
			entry["partial_line_omitted"] = partial
			inventory = append(inventory, entry)
			if e = add(group.dir+"/"+name, data); e != nil {
				return e
			}
		}
	}
	if info == nil {
		info = map[string]any{"exporter_version": ServerVersion, "created_at": time.Now().Format(time.RFC3339Nano), "os": runtime.GOOS, "arch": runtime.GOARCH, "mode": "offline_or_live_file_snapshot"}
	}
	b, e := json.MarshalIndent(info, "", "  ")
	if e != nil {
		return e
	}
	if e = add("server-info.json", b); e != nil {
		return e
	}
	b, e = json.MarshalIndent(map[string]any{"schema": "speedbackup.debug.v1", "files": inventory, "snapshot": "Live files are read to their observed length; concurrent rotation can omit a file. Inspect errors, truncation, partial-line and dropped-record counters.", "excluded": []string{"configuration/auth files", "passwords/tokens/cookies/Authorization headers", "request/response bodies", "backup contents", "environment and process arguments"}}, "", "  ")
	if e != nil {
		return e
	}
	if e = add("bundle-manifest.json", b); e != nil {
		return e
	}
	if e = add("README.txt", []byte("SpeedBackup Server debug bundle\nserver-info.json: version, runtime and active transfers (web export).\nlogs/server.jsonl*: request method/path, source/destination, status, byte counts, elapsed time, errors; web UI errors.\naudit/events.jsonl*: completed operations and lifecycle history.\nTimestamps include timezone; request_id links requests and operations.\nLogs contain file paths, account names and client IP addresses. No backup file content or authentication configuration is collected.\nCheck bundle-manifest.json for missing/truncated files. Retention is bounded; older history may have rotated out.\n")); e != nil {
		return e
	}
	return z.Close()
}

var diagnosticFrame = regexp.MustCompile(`(?:app|dav_monitor|dav_config|dav_editor|preferences)\.js:\d+:\d+`)

func (s *HTTPServer) adminClientError(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Page    string `json:"page"`
		Name    string `json:"name"`
		Message string `json:"message"`
		Stack   string `json:"stack"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if len(v.Message) > 512 || len(v.Stack) > 8192 || len(v.Name) > 48 || !strings.Contains("|webdav|dashboard|profiles|sessions|events|settings|", "|"+v.Page+"|") {
		writeErr(w, 400, "invalid UI error report")
		return
	}
	frames := diagnosticFrame.FindAllString(v.Stack, 16)
	s.store.diagnostics.append(map[string]any{"kind": "web_ui_error", "time": time.Now().Format(time.RFC3339Nano), "page": v.Page, "name": v.Name, "message": v.Message, "frames": frames, "request_id": requestTraceID(r)})
	writeJSON(w, 200, map[string]bool{"recorded": true})
}
