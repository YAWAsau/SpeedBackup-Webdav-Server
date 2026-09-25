package sbserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func (d *davService) accountDirectory(a davAccount) string {
	if a.Directory != "" {
		return a.Directory
	}
	return filepath.Join(d.store.Root, "webdav", a.Username)
}
func containsDirectory(parent, child string) bool {
	if runtime.GOOS == "windows" {
		parent = strings.ToLower(parent)
		child = strings.ToLower(child)
	}
	rel, e := filepath.Rel(parent, child)
	return e == nil && (rel == "." || (!filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
func canonicalDirectory(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("please enter an absolute server directory")
	}
	p, e := filepath.EvalSymlinks(filepath.Clean(p))
	if e != nil {
		return "", fmt.Errorf("directory does not exist or is inaccessible")
	}
	info, e := os.Stat(p)
	if e != nil || !info.IsDir() {
		return "", fmt.Errorf("please select an existing directory")
	}
	return p, nil
}
func (d *davService) privateDirectory(p string) bool {
	private, e := filepath.EvalSymlinks(d.store.internal())
	if e != nil {
		return true
	}
	return containsDirectory(private, p)
}
func (d *davService) validateDirectory(p, user string) (string, error) {
	dir, e := canonicalDirectory(p)
	if e != nil {
		return "", e
	}
	private, e := filepath.EvalSymlinks(d.store.internal())
	if e != nil {
		return "", e
	}
	if containsDirectory(dir, private) || containsDirectory(private, dir) {
		return "", fmt.Errorf("share directory must not expose server configuration")
	}
	// Distinct accounts keep distinct roots, including aliases through symlinks.
	entries, e := os.ReadDir(d.store.internal("webdav-users"))
	if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	for _, v := range entries {
		if !v.IsDir() || v.Name() == user {
			continue
		}
		a, _, e := d.load(v.Name())
		if e != nil {
			return "", fmt.Errorf("cannot verify other account directories")
		}
		other, e := canonicalDirectory(d.accountDirectory(a))
		if e != nil {
			continue
		}
		if containsDirectory(other, dir) || containsDirectory(dir, other) {
			return "", fmt.Errorf("directory overlaps another backup account")
		}
	}
	f, e := os.Open(dir)
	if e != nil {
		return "", fmt.Errorf("server cannot read this directory")
	}
	_, e = f.Readdirnames(1)
	_ = f.Close()
	if e != nil && e != io.EOF {
		return "", fmt.Errorf("server cannot list this directory")
	}
	probe, e := os.CreateTemp(dir, davStaging+"permission-*")
	if e != nil {
		return "", fmt.Errorf("server service account cannot write this directory")
	}
	name := probe.Name()
	ce := probe.Close()
	re := os.Remove(name)
	if ce != nil || re != nil {
		return "", fmt.Errorf("server cannot clean up directory permission probe")
	}
	return dir, nil
}

type directoryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (d *davService) directories(w http.ResponseWriter, r *http.Request) {
	selected := r.URL.Query().Get("path")
	roots := directoryRoots()
	if selected == "" {
		writeJSON(w, 200, map[string]any{"path": "", "parent": "", "roots": roots, "directories": []directoryEntry{}})
		return
	}
	dir, e := canonicalDirectory(selected)
	if e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	if d.privateDirectory(dir) {
		writeErr(w, 403, "server configuration directories are private")
		return
	}
	f, e := os.Open(dir)
	if e != nil {
		writeErr(w, 403, "server cannot open this directory")
		return
	}
	defer f.Close()
	// Bound the response and the work for very large backup directories.
	items, e := f.ReadDir(2001)
	if e != nil && e != io.EOF {
		writeErr(w, 403, "server cannot list this directory")
		return
	}
	truncated := len(items) > 2000
	if truncated {
		items = items[:2000]
	}
	dirs := []directoryEntry{}
	for _, v := range items {
		if strings.HasPrefix(v.Name(), davStaging) {
			continue
		}
		p := filepath.Join(dir, v.Name())
		resolved, e := canonicalDirectory(p)
		if e == nil && !d.privateDirectory(resolved) {
			dirs = append(dirs, directoryEntry{v.Name(), resolved})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })
	writeJSON(w, 200, map[string]any{"path": dir, "parent": filepath.Dir(dir), "roots": roots, "directories": dirs, "truncated": truncated})
}
