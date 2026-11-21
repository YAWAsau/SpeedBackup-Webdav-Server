package sbserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
)

// Storage keys never change. One atomically published manifest changes the
// login name without moving backup files or rewriting credential records.
func (d *davService) accountAliases() (map[string]string, string, error) {
	aliases := map[string]string{}
	p, err := latestVersionFile(d.store.internal("webdav-account-names"))
	if errors.Is(err, os.ErrNotExist) {
		return aliases, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, "", err
	}
	if err = json.Unmarshal(b, &aliases); err != nil {
		return nil, "", err
	}
	seen := map[string]bool{}
	for key, name := range aliases {
		if !davUsername.MatchString(key) || !davUsername.MatchString(name) || seen[name] {
			return nil, "", os.ErrInvalid
		}
		seen[name] = true
	}
	return aliases, p, nil
}

func (d *davService) accountKey(user string) (string, string, error) {
	if !davUsername.MatchString(user) {
		return "", "", os.ErrNotExist
	}
	aliases, version, err := d.accountAliases()
	if err != nil {
		return "", "", err
	}
	for key, name := range aliases {
		if name == user {
			return key, version, nil
		}
	}
	if _, renamed := aliases[user]; renamed {
		return "", "", os.ErrNotExist
	}
	return user, version, nil
}

func (d *davService) accountNames() ([]string, error) {
	aliases, _, err := d.accountAliases()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d.store.internal("webdav-users"))
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if renamed, ok := aliases[name]; ok {
			name = renamed
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (d *davService) renameAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		NewUsername string `json:"new_username"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if json.NewDecoder(r.Body).Decode(&req) != nil || !davUsername.MatchString(req.Username) || !davUsername.MatchString(req.NewUsername) {
		writeErr(w, 400, "invalid account name")
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	a, _, err := d.load(req.Username)
	if err != nil {
		writeErr(w, 404, "account not found")
		return
	}
	if req.Username == req.NewUsername {
		writeJSON(w, 200, map[string]string{"username": req.NewUsername})
		return
	}
	names, err := d.accountNames()
	if err != nil {
		writeErr(w, 500, "cannot read account names")
		return
	}
	for _, name := range names {
		if name == req.NewUsername {
			writeErr(w, 409, "account name already exists")
			return
		}
	}
	key, _, err := d.accountKey(req.Username)
	if err != nil {
		writeErr(w, 500, "cannot resolve account")
		return
	}
	// A different account's historical storage key stays reserved. The original
	// account may rename back to its own key.
	if req.NewUsername != key {
		if _, err = os.Stat(d.store.internal("webdav-users", req.NewUsername)); err == nil {
			writeErr(w, 409, "account name is reserved by an existing account")
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			writeErr(w, 500, "cannot verify account name")
			return
		}
	}
	aliases, _, err := d.accountAliases()
	if err != nil {
		writeErr(w, 500, "cannot read account names")
		return
	}
	aliases[key] = req.NewUsername
	dir := d.store.internal("webdav-account-names")
	if err = os.MkdirAll(dir, 0700); err == nil {
		var n uint64
		n, err = nextVersion(dir)
		if err == nil {
			err = writeImmutableJSON(filepath.Join(dir, fmt.Sprintf("%020d.json", n)), aliases)
		}
	}
	if err != nil {
		writeErr(w, 500, "cannot save account name")
		return
	}
	delete(d.cache, req.Username)
	delete(d.cache, req.NewUsername)
	writeJSON(w, 200, map[string]string{"username": req.NewUsername, "directory": d.accountDirectory(a)})
}
