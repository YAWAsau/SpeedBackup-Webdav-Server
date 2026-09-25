package sbserver

// The wire protocol is golang.org/x/net/webdav. This file adds account isolation,
// durable uploads and management; it does not introduce a client protocol.
import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/webdav"
)

const davPrefix = "/dav/"
const davPublicPrefix = "/dav-public/"
const davStaging = ".speedbackup-upload-"

var davUsername = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,47}$`)

// These namespaces belong to management and backwards-compatible DAV mounts.
func davRootReserved(name string) bool {
	first, _, _ := strings.Cut(strings.TrimPrefix(path.Clean("/"+name), "/"), "/")
	return first == "api" || first == "web" || first == "dav" || first == "dav-public"
}

type davAccount struct {
	Username     string `json:"username"`
	Salt         string `json:"salt"`
	PasswordHash string `json:"password_hash"`
	Disabled     bool   `json:"disabled"`
	Anonymous    bool   `json:"anonymous,omitempty"`
	Directory    string `json:"directory,omitempty"`
}
type davService struct {
	activity davActivity
	store    *Store
	mu       sync.Mutex
	gates    map[string]*davMutationGate
	locks    map[string]webdav.LockSystem
	cache    map[string]davAuthCache
}
type davAuthCache struct {
	credential [32]byte
	version    string
	until      time.Time
}

func newDAVService(store *Store) *davService {
	return &davService{store: store, gates: map[string]*davMutationGate{}, locks: map[string]webdav.LockSystem{}, cache: map[string]davAuthCache{}}
}
func (d *davService) load(user string) (davAccount, string, error) {
	var a davAccount
	if !davUsername.MatchString(user) {
		return a, "", os.ErrNotExist
	}
	p, e := latestVersionFile(d.store.internal("webdav-users", user))
	if e != nil {
		return a, "", e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return a, "", e
	}
	e = json.Unmarshal(b, &a)
	if a.Username != user || (!a.Anonymous && (a.Salt == "" || a.PasswordHash == "")) {
		return a, "", os.ErrInvalid
	}
	return a, p, e
}
func (d *davService) authenticate(user, password string) bool {
	a, version, e := d.load(user)
	if e != nil || a.Disabled {
		return false
	}
	digest := sha256.Sum256([]byte(user + "\x00" + password))
	d.mu.Lock()
	cached, ok := d.cache[user]
	d.mu.Unlock()
	if ok && cached.version == version && time.Now().Before(cached.until) && subtle.ConstantTimeCompare(cached.credential[:], digest[:]) == 1 {
		return true
	}
	salt, e := hex.DecodeString(a.Salt)
	if e != nil {
		return false
	}
	hash, e := pbkdf2.Key(sha256.New, password, salt, 210000, 32)
	if e != nil {
		return false
	}
	want, e := hex.DecodeString(a.PasswordHash)
	if e != nil || subtle.ConstantTimeCompare(hash, want) != 1 {
		return false
	}
	d.mu.Lock()
	d.cache[user] = davAuthCache{digest, version, time.Now().Add(5 * time.Minute)}
	d.mu.Unlock()
	return true
}
func (d *davService) resources(user string) (*davMutationGate, webdav.LockSystem) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.gates[user] == nil {
		d.gates[user] = &davMutationGate{}
		d.locks[user] = webdav.NewMemLS()
	}
	return d.gates[user], d.locks[user]
}

// Never choose an arbitrary directory when multiple anonymous shares exist.
func (d *davService) rootAnonymousUser() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	entries, err := os.ReadDir(d.store.internal("webdav-users"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	user := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		a, _, err := d.load(entry.Name())
		if err != nil {
			return "", err
		}
		if a.Anonymous && !a.Disabled {
			if user != "" {
				return "", nil
			}
			user = a.Username
		}
	}
	return user, nil
}

func (d *davService) admin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		list := []map[string]any{}
		entries, e := os.ReadDir(d.store.internal("webdav-users"))
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			writeErr(w, 500, "cannot read WebDAV accounts")
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			a, _, e := d.load(entry.Name())
			if e != nil {
				writeErr(w, 500, "invalid WebDAV account configuration")
				return
			}
			list = append(list, map[string]any{"username": a.Username, "disabled": a.Disabled, "anonymous": a.Anonymous, "anonymous_path": davPublicPrefix + a.Username + "/", "directory": d.accountDirectory(a), "custom_directory": a.Directory != ""})
		}
		rootUser, e := d.rootAnonymousUser()
		if e != nil {
			writeErr(w, 500, "cannot read WebDAV accounts")
			return
		}
		writeJSON(w, 200, map[string]any{"path": "/", "root_anonymous_user": rootUser, "users": list, "storage": filepath.Join(d.store.Root, "webdav"), "protocol": "WebDAV"})
		return
	}
	var req struct {
		Username  string  `json:"username"`
		Password  string  `json:"password"`
		Disabled  bool    `json:"disabled"`
		Anonymous *bool   `json:"anonymous"`
		Directory *string `json:"directory"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if e := json.NewDecoder(r.Body).Decode(&req); e != nil || !davUsername.MatchString(req.Username) {
		writeErr(w, 400, "username: 1-48 lowercase letters, digits, _ or -")
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	a, _, e := d.load(req.Username)
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		writeErr(w, 500, "cannot load WebDAV account")
		return
	}
	if req.Anonymous != nil {
		a.Anonymous = *req.Anonymous
	}
	if strings.TrimSpace(req.Password) == "" && (req.Password != "" || (!a.Anonymous && a.PasswordHash == "")) {
		writeErr(w, 400, "password must not be blank")
		return
	}
	if req.Password != "" {
		salt := make([]byte, 16)
		if _, e = rand.Read(salt); e != nil {
			writeErr(w, 500, "random source failed")
			return
		}
		hash, e := pbkdf2.Key(sha256.New, req.Password, salt, 210000, 32)
		if e != nil {
			writeErr(w, 500, "password derivation failed")
			return
		}
		a.Username = req.Username
		a.Salt = hex.EncodeToString(salt)
		a.PasswordHash = hex.EncodeToString(hash)
	}
	a.Username = req.Username
	a.Disabled = req.Disabled
	if req.Directory != nil {
		if strings.TrimSpace(*req.Directory) == "" {
			a.Directory = ""
		} else {
			var directory string
			directory, e = d.validateDirectory(*req.Directory, req.Username)
			if e != nil {
				writeErr(w, 400, e.Error())
				return
			}
			a.Directory = directory
		}
	}
	dir := d.store.internal("webdav-users", req.Username)
	if e = os.MkdirAll(dir, 0700); e != nil {
		writeErr(w, 500, "cannot create account configuration")
		return
	}
	if a.Directory == "" {
		e = os.MkdirAll(d.accountDirectory(a), 0700)
	}
	if e != nil {
		writeErr(w, 500, "cannot create account storage")
		return
	}
	if a.Directory == "" {
		if _, e = d.validateDirectory(d.accountDirectory(a), a.Username); e != nil {
			writeErr(w, 400, e.Error())
			return
		}
	}
	n, e := nextVersion(dir)
	if e == nil {
		e = writeImmutableJSON(filepath.Join(dir, fmt.Sprintf("%020d.json", n)), a)
	}
	if e != nil {
		writeErr(w, 500, "cannot save WebDAV account")
		return
	}
	delete(d.cache, req.Username)
	writeJSON(w, 200, map[string]any{"username": req.Username, "disabled": a.Disabled, "path": "/"})
}

func (d *davService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Advertise the DAV protocol on known share roots without revealing accounts
	// or relaxing authentication. Anonymous discovery still receives 401.
	if r.Method == "OPTIONS" && (r.URL.Path == "/" || r.URL.Path == davPrefix || r.URL.Path == strings.TrimSuffix(davPrefix, "/")) {
		w.Header().Set("DAV", "1, 2")
	}
	prefix := "/"
	if r.URL.Path == strings.TrimSuffix(davPrefix, "/") || strings.HasPrefix(r.URL.Path, davPrefix) {
		prefix = davPrefix
	}
	public := strings.HasPrefix(r.URL.Path, davPublicPrefix)
	user, password, ok := r.BasicAuth()
	if public {
		user, _, _ = strings.Cut(strings.TrimPrefix(r.URL.Path, davPublicPrefix), "/")
		a, _, err := d.load(user)
		if err != nil || a.Disabled || !a.Anonymous {
			http.NotFound(w, r)
			return
		}
		prefix = davPublicPrefix + user + "/"
	} else if prefix == "/" && (r.Header.Get("Authorization") == "" || ok && user == "" && password == "") {
		var err error
		user, err = d.rootAnonymousUser()
		if err != nil {
			http.Error(w, "account unavailable", 503)
			return
		}
		public = user != ""
	}
	if !public && (!ok || !d.authenticate(user, password)) {
		w.Header().Set("WWW-Authenticate", `Basic realm="SpeedBackup WebDAV", charset="UTF-8"`)
		http.Error(w, "authentication required", 401)
		return
	}
	if trace := requestTrace(r); trace != nil {
		trace.User = user
	}
	if prefix == "/" && davRootReserved(r.URL.Path) {
		http.Error(w, "reserved server path; use the share-specific URL", 403)
		return
	}
	// Existing script clients stat the configured base with its trailing slash
	// removed. Normalize only the share root internally, without an HTTP redirect.
	if r.URL.Path == strings.TrimSuffix(prefix, "/") {
		r = r.Clone(r.Context())
		r.URL.Path = prefix
		r.URL.RawPath = ""
	}
	if davTransfersData(r.Method) || r.Method == "DELETE" || r.Method == "MOVE" || r.Method == "COPY" || r.Method == "MKCOL" {
		expected := int64(-1)
		if r.Method == "PUT" {
			expected = r.ContentLength
		}
		// Record only a validated share-relative destination, never credentials
		// or arbitrary external URLs supplied in a rejected request header.
		destination := ""
		if r.Method == "MOVE" || r.Method == "COPY" {
			if u, e := url.Parse(r.Header.Get("Destination")); e == nil && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Host == "" || strings.EqualFold(u.Host, r.Host)) && strings.HasPrefix(u.Path, prefix) {
				if n, e := davName(strings.TrimPrefix(u.Path, prefix)); e == nil {
					destination = n
				}
			}
		}
		v := d.activity.start(user, r.Method, strings.TrimPrefix(r.URL.Path, prefix), expected, destination, requestTraceID(r))
		wm := &davWriteMeter{ResponseWriter: w, a: &d.activity, v: v, download: r.Method == "GET"}
		rm := &davReadMeter{ReadCloser: r.Body, a: &d.activity, v: v}
		if r.Method == "PUT" {
			r.Body = rm
		}
		w = wm
		defer func() {
			err := wm.err
			if err == nil {
				err = rm.err
			}
			if err == nil {
				err = r.Context().Err()
			}
			status := wm.status
			if status == 0 {
				status = 200
			}
			result := d.activity.finish(v, status, err)
			_ = d.store.AppendEvent(AuditEvent{Unix: nowUnix(), Event: "webdav_transfer", Message: result.State, Details: result})
		}()
	}
	if _, e := davName(strings.TrimPrefix(r.URL.Path, prefix)); e != nil {
		http.Error(w, "invalid path", 403)
		return
	}
	names := []string{strings.TrimPrefix(r.URL.Path, prefix)}
	// A browser GET may contain //, but never redirect a streaming PUT/MOVE body.
	if dst := r.Header.Get("Destination"); dst != "" {
		u, e := url.Parse(dst)
		if e != nil || u.RawQuery != "" || u.Fragment != "" || u.User != nil || (u.Host != "" && !strings.EqualFold(u.Host, r.Host)) || !strings.HasPrefix(u.Path, prefix) {
			http.Error(w, "invalid destination", 400)
			return
		}
		if _, e = davName(strings.TrimPrefix(u.Path, prefix)); e != nil {
			http.Error(w, "invalid destination", 403)
			return
		}
		if prefix == "/" && davRootReserved(u.Path) {
			http.Error(w, "reserved destination; use the share-specific URL", 400)
			return
		}
		names = append(names, strings.TrimPrefix(u.Path, prefix))
	}
	gate, locks := d.resources(user)
	// RFC 4918 defaults Overwrite to T. x/net's MOVE still tests == "T",
	// unlike COPY; normalize the omitted header as rclone also does (#66059).
	if (r.Method == "COPY" || r.Method == "MOVE") && r.Header.Get("Overwrite") == "" {
		r.Header.Set("Overwrite", "T")
	}
	mutating := r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && r.Method != "PROPFIND"
	account, version, e := d.load(user)
	if e != nil {
		http.Error(w, "account unavailable", 503)
		return
	}
	if account.Disabled || (public && !account.Anonymous) {
		http.NotFound(w, r)
		return
	}
	if mutating {
		release, err := gate.acquireNames(r.Context(), d.accountDirectory(account), names)
		if err != nil {
			http.Error(w, "cannot acquire storage paths", 503)
			return
		}
		defer release()
		// An administrator may change a share while this request waits. Never
		// publish into a different directory using locks for the old directory.
		_, current, err := d.load(user)
		if err != nil || current != version {
			http.Error(w, "account changed; retry request", 503)
			return
		}
	}
	root, e := os.OpenRoot(d.accountDirectory(account))
	if e != nil {
		http.Error(w, "storage unavailable", 503)
		return
	}
	defer root.Close()
	fs := &davFS{root: root, method: r.Method, staged: map[string]string{}}
	defer fs.cleanup()
	h := &webdav.Handler{Prefix: prefix, FileSystem: fs, LockSystem: locks, Logger: func(r *http.Request, err error) { requestTraceError(r, err) }}
	if !mutating {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
		return
	}
	// Only mutation responses are buffered (small status/XML); file bodies stream
	// directly to disk. Never report success before fsync and final publication.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	if rr.Code >= 200 && rr.Code < 300 {
		if e = r.Context().Err(); e == nil {
			e = fs.commit()
		}
		if e != nil {
			requestTraceError(r, e)
			http.Error(w, "file publication failed", 500)
			return
		}
	}
	for k, v := range rr.Header() {
		w.Header()[k] = v
	}
	w.WriteHeader(rr.Code)
	_, _ = io.Copy(w, rr.Body)
	// No post-MOVE source stat: it has ceased to exist. Unlike rclone's old
	// X-OC-Mtime postprocess, normal successful MOVE needs no extra lookup.
}

func davName(name string) (string, error) {
	if strings.ContainsAny(name, "\\\x00:") {
		return "", os.ErrPermission
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || strings.HasPrefix(strings.ToLower(part), davStaging) {
			return "", os.ErrPermission
		}
	}
	return path.Clean("/" + name)[1:], nil
}
func localDAVName(name string) (string, error) {
	n, e := davName(name)
	if n == "" {
		n = "."
	}
	return n, e
}

type davFS struct {
	root   *os.Root
	method string
	staged map[string]string
}

func (f *davFS) cleanup() {
	for _, tmp := range f.staged {
		_ = f.root.Remove(tmp)
	}
}
func (f *davFS) commit() error {
	for name, tmp := range f.staged {
		if e := f.root.Rename(tmp, name); e != nil {
			return e
		}
		delete(f.staged, name)
	}
	return nil
}
func (f *davFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	n, e := localDAVName(name)
	if e != nil {
		return e
	}
	return f.root.Mkdir(n, 0700)
}
func (f *davFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	n, e := localDAVName(name)
	if e != nil {
		return nil, e
	}
	if tmp, ok := f.staged[n]; ok {
		n = tmp
	}
	i, e := f.root.Stat(n)
	if e != nil {
		return nil, e
	}
	return davInfo{i, path.Base(name)}, nil
}
func (f *davFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	n, e := localDAVName(name)
	if e != nil {
		return nil, e
	}
	actual := n
	writing := flag&(os.O_WRONLY|os.O_RDWR) != 0
	if writing {
		if n == "." {
			return nil, os.ErrPermission
		}
		id := make([]byte, 16)
		if _, e = rand.Read(id); e != nil {
			return nil, e
		}
		actual = path.Join(path.Dir(n), davStaging+hex.EncodeToString(id))
		flag = os.O_CREATE | os.O_EXCL | os.O_RDWR
		perm = 0600
	} else if tmp, ok := f.staged[n]; ok {
		actual = tmp
	}
	file, e := f.root.OpenFile(actual, flag, perm)
	if e != nil {
		return nil, e
	}
	if writing {
		f.staged[n] = actual
	}
	return &davFile{File: file, writing: writing, name: path.Base(n)}, nil
}
func (f *davFS) RemoveAll(ctx context.Context, name string) error {
	n, e := localDAVName(name)
	if e != nil {
		return e
	}
	if n == "." {
		return os.ErrPermission
	}
	if f.method == "MOVE" || f.method == "COPY" {
		info, e := f.root.Stat(n)
		if e != nil {
			return e
		}
		if info.IsDir() {
			return os.ErrPermission
		}
		// File replacement happens at Rename/commit; do not delete good old bytes
		// before the new upload/copy has completed. Directory replacement is refused.
		return nil
	}
	return f.root.RemoveAll(n)
}
func (f *davFS) Rename(ctx context.Context, old, new string) error {
	a, e := localDAVName(old)
	if e != nil {
		return e
	}
	b, e := localDAVName(new)
	if e != nil {
		return e
	}
	if a == "." || b == "." {
		return os.ErrPermission
	}
	return f.root.Rename(a, b)
}

type davFile struct {
	*os.File
	writing bool
	name    string
}

func (f *davFile) Close() error {
	if f.writing {
		if e := f.File.Sync(); e != nil {
			_ = f.File.Close()
			return e
		}
	}
	return f.File.Close()
}
func (f *davFile) Stat() (os.FileInfo, error) {
	i, e := f.File.Stat()
	if e != nil {
		return nil, e
	}
	return davInfo{i, f.name}, nil
}
func (f *davFile) Readdir(count int) ([]os.FileInfo, error) {
	out := []os.FileInfo{}
	for {
		batch := 256
		if count > 0 && count-len(out) < batch {
			batch = count - len(out)
		}
		items, e := f.File.Readdir(batch)
		for _, i := range items {
			if !strings.HasPrefix(strings.ToLower(i.Name()), davStaging) {
				out = append(out, davInfo{i, i.Name()})
			}
		}
		if e != nil {
			if e == io.EOF && (count <= 0 || len(out) > 0) {
				e = nil
			}
			return out, e
		}
		if count > 0 && len(out) >= count {
			return out, nil
		}
	}
}

type davInfo struct {
	os.FileInfo
	name string
}

func (i davInfo) Name() string { return i.name }

// Backup payloads have known binary types. Avoid opening each archive merely
// to sniff its first 512 bytes during PROPFIND. Unknown types retain x/net's
// content detection; directory contents are never cached.
func (i davInfo) ContentType(ctx context.Context) (string, error) {
	if i.IsDir() {
		return "", webdav.ErrNotImplemented
	}
	name := strings.ToLower(i.name)
	if strings.Contains(name, ".part.") || strings.HasSuffix(name, ".part") {
		return "application/octet-stream", nil
	}
	switch path.Ext(name) {
	case ".zst", ".zstd", ".lz4", ".br", ".xz", ".bin", ".apk":
		return "application/octet-stream", nil
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		return contentType, nil
	}
	return "", webdav.ErrNotImplemented
}
