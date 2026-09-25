package sbserver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func awaitDAV(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("DAV operation did not progress")
	}
}
func waitDAVQueue(t *testing.T, g *davMutationGate, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		n := len(g.waiting)
		g.mu.Unlock()
		if n == count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("DAV queue did not reach %d", count)
}
func TestDAVPathGateFairnessAndCancellation(t *testing.T) {
	g := &davMutationGate{}
	release, err := g.acquire(context.Background(), []string{"/share/a/one"})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	parent := make(chan struct{})
	unblock := make(chan struct{})
	defer close(unblock)
	go func() { r, _ := g.acquire(context.Background(), []string{"/share/a"}); close(parent); <-unblock; r() }()
	waitDAVQueue(t, g, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	child := make(chan error, 1)
	go func() {
		r, e := g.acquire(ctx, []string{"/share/a/two"})
		if r != nil {
			r()
		}
		child <- e
	}()
	waitDAVQueue(t, g, 2)
	// An earlier parent waiter prevents a later child starving a rename/delete.
	select {
	case <-child:
		t.Fatal("child overtook parent")
	default:
	}
	other, e := g.acquire(context.Background(), []string{"/share/b"})
	if e != nil {
		t.Fatal(e)
	}
	other()
	cancel()
	select {
	case e := <-child:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter stuck")
	}
	release()
	awaitDAV(t, parent)
}

func TestDAVPhysicalPathAliases(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	a, e := davMutationPaths(root, "real/new/file")
	if e != nil {
		t.Fatal(e)
	}
	parent, e := davMutationPaths(root, "real")
	if e != nil || !davPathsOverlap(a, parent) {
		t.Fatal("parent failed to cover children", e)
	}
	if runtime.GOOS == "windows" {
		b, e := davMutationPaths(root, "REAL/new/FILE")
		if e != nil || !davPathsOverlap(a, b) {
			t.Fatal("case alias not covered", e)
		}
	}
	if e := os.Symlink(real, filepath.Join(root, "alias")); e != nil {
		t.Skip("symlinks unavailable: ", e)
	}
	b, e := davMutationPaths(root, "alias/new/file")
	if e != nil || !davPathsOverlap(a, b) {
		t.Fatal("symlink alias not covered", e)
	}
}

type heldDAVBody struct {
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
	data    *strings.Reader
}

func (b *heldDAVBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.data.Read(p)
}
func (b *heldDAVBody) Close() error { return nil }
func newConcurrencyDAV(t *testing.T) (*davService, string) {
	t.Helper()
	root := t.TempDir()
	store, e := NewStore(root)
	if e != nil {
		t.Fatal(e)
	}
	d := newDAVService(store)
	r := httptest.NewRequest("POST", "http://local/api", strings.NewReader(`{"username":"phone","anonymous":true}`))
	w := httptest.NewRecorder()
	d.admin(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	return d, filepath.Join(root, "webdav", "phone")
}
func TestDAVParallelUploadAndIndependentMove(t *testing.T) {
	d, root := newConcurrencyDAV(t)
	if e := os.Mkdir(filepath.Join(root, "folder"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "ready.part"), []byte("ready"), 0600); e != nil {
		t.Fatal(e)
	}
	release := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	done := make(chan int, 2)
	for _, name := range []string{"folder/one", "folder/two"} {
		body := &heldDAVBody{entered: make(chan struct{}), release: release, data: strings.NewReader(name)}
		go func() {
			r := httptest.NewRequest("PUT", "http://local/"+name, body)
			w := httptest.NewRecorder()
			d.ServeHTTP(w, r)
			done <- w.Code
		}()
		// The second independent file must begin reading before the first ends.
		awaitDAV(t, body.entered)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("MOVE", "http://local/ready.part", nil)
	r.Header.Set("Destination", "http://local/ready")
	moved := make(chan struct{})
	go func() { d.ServeHTTP(w, r); close(moved) }()
	awaitDAV(t, moved)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	// A parent MOVE waits for both staged uploads and publishes all children.
	w = httptest.NewRecorder()
	r = httptest.NewRequest("MOVE", "http://local/folder", nil)
	r.Header.Set("Destination", "http://local/published")
	parentW, parentR := w, r
	moved = make(chan struct{})
	parentDone := moved
	go func() { d.ServeHTTP(parentW, parentR); close(parentDone) }()
	g, _ := d.resources("phone")
	waitDAVQueue(t, g, 1)
	finish()
	for range 2 {
		if code := <-done; code != 201 {
			t.Fatal("PUT", code)
		}
	}
	awaitDAV(t, moved)
	if w.Code != 201 {
		t.Fatal("MOVE", w.Code, w.Body.String())
	}
	for _, name := range []string{"one", "two"} {
		b, e := os.ReadFile(filepath.Join(root, "published", name))
		if e != nil || string(b) != "folder/"+name {
			t.Fatal("publication corrupted", e, string(b))
		}
	}
}

func TestDAVQueuedAccountChangeAndAbort(t *testing.T) {
	d, root := newConcurrencyDAV(t)
	g, _ := d.resources("phone")
	keys, e := davMutationPaths(root, "backup.zst")
	if e != nil {
		t.Fatal(e)
	}
	release, e := g.acquire(context.Background(), keys)
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "http://local/backup.zst", strings.NewReader("new"))
	done := make(chan struct{})
	go func() { d.ServeHTTP(w, r); close(done) }()
	waitDAVQueue(t, g, 1)
	aw := httptest.NewRecorder()
	d.admin(aw, httptest.NewRequest("POST", "http://local/api", strings.NewReader(`{"username":"phone","disabled":true}`)))
	if aw.Code != 200 {
		t.Fatal(aw.Body.String())
	}
	release()
	awaitDAV(t, done)
	if w.Code != 503 {
		t.Fatalf("changed account accepted: %d", w.Code)
	}
	if _, e := os.Stat(filepath.Join(root, "backup.zst")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("queued upload published", e)
	}
}

func TestDAVBatchListingAndContentType(t *testing.T) {
	root := t.TempDir()
	for i := range 270 {
		if e := os.WriteFile(filepath.Join(root, fmt.Sprintf("%03d.zst", i)), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	os.WriteFile(filepath.Join(root, davStaging+"hidden"), nil, 0600)
	r, e := os.OpenRoot(root)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	fs := &davFS{root: r, staged: map[string]string{}}
	for _, count := range []int{-1, 1, 17, 256} {
		f, e := fs.OpenFile(context.Background(), "/", os.O_RDONLY, 0)
		if e != nil {
			t.Fatal(e)
		}
		seen := map[string]bool{}
		for {
			items, e := f.Readdir(count)
			for _, i := range items {
				if seen[i.Name()] || strings.HasPrefix(i.Name(), davStaging) {
					t.Fatal("duplicate/hidden entry")
				}
				seen[i.Name()] = true
			}
			if count <= 0 || e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
		}
		f.Close()
		if len(seen) != 270 {
			t.Fatal("missing entries", count, len(seen))
		}
	}
	i, e := fs.Stat(context.Background(), "000.zst")
	if e != nil {
		t.Fatal(e)
	}
	if ct, e := i.(davInfo).ContentType(context.Background()); e != nil || ct != "application/octet-stream" {
		t.Fatal(ct, e)
	}
}

func TestDAVAtomicProgressAndEventWait(t *testing.T) {
	a := &davActivity{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	idle := make(chan error, 1)
	go func() { idle <- a.wait(ctx, 0, time.Millisecond, time.Minute) }()
	select {
	case <-idle:
		t.Fatal("idle watch polled without event")
	case <-time.After(20 * time.Millisecond):
	}
	v := a.start("phone", "PUT", "backup/app/user.zst", 32000)
	select {
	case e := <-idle:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("event failed to wake watch")
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for range 1000 {
				a.add(v, 1)
				_ = a.snapshot()
			}
		})
	}
	wg.Wait()
	_, rev := a.snapshotRevision()
	e := a.wait(context.Background(), rev, time.Millisecond, time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	got := a.finish(v, 201, nil)
	if got.Bytes != 32000 || got.State != "transfer_complete" {
		t.Fatalf("%+v", got)
	}
	_, rev = a.snapshotRevision()
	cancel()
	if e := a.wait(ctx, rev, time.Second, time.Minute); !errors.Is(e, context.Canceled) {
		t.Fatal("cancel failed", e)
	}
}

func TestDAVTruncatedHTTPUploadKeepsOldBackup(t *testing.T) {
	// Exercise the real HTTP server's Content-Length handling, not just a mock
	// reader. Go's transport cuts the connection when the body ends too early.
	s, root := davTestServer(t)
	old := filepath.Join(root, "webdav", "phone", "backup.zst")
	if e := os.WriteFile(old, []byte("old-good"), 0600); e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("PUT", s.URL+"/backup.zst", nil)
	r.SetBasicAuth("phone", "correct-password-123")
	c, e := net.DialTimeout("tcp", strings.TrimPrefix(s.URL, "http://"), time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(c, "PUT /backup.zst HTTP/1.1\r\nHost: %s\r\nAuthorization: %s\r\nContent-Length: 100\r\nConnection: close\r\n\r\nshort", r.Host, r.Header.Get("Authorization"))
	c.(*net.TCPConn).CloseWrite()
	resp, e := http.ReadResponse(bufio.NewReader(c), r)
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 400 {
		t.Fatal("truncated upload accepted", resp.StatusCode)
	}
	files, e := os.ReadDir(filepath.Dir(old))
	if e != nil || len(files) != 1 {
		t.Fatal("staging file leaked", e, len(files))
	}
	b, e := os.ReadFile(old)
	if e != nil || string(b) != "old-good" {
		t.Fatal("partial backup published", e, string(b))
	}
}

func TestDAVSamePathUploadsPublishInOrder(t *testing.T) {
	d, root := newConcurrencyDAV(t)
	g, _ := d.resources("phone")
	firstRelease, secondRelease := make(chan struct{}), make(chan struct{})
	var firstOnce, secondOnce sync.Once
	endFirst := func() { firstOnce.Do(func() { close(firstRelease) }) }
	endSecond := func() { secondOnce.Do(func() { close(secondRelease) }) }
	defer endFirst()
	defer endSecond()
	start := func(value string, release <-chan struct{}) (chan struct{}, chan int) {
		body := &heldDAVBody{entered: make(chan struct{}), release: release, data: strings.NewReader(value)}
		done := make(chan int, 1)
		go func() {
			w := httptest.NewRecorder()
			d.ServeHTTP(w, httptest.NewRequest("PUT", "http://local/same.zst", body))
			done <- w.Code
		}()
		return body.entered, done
	}
	entered, first := start("first", firstRelease)
	awaitDAV(t, entered)
	entered, second := start("second", secondRelease)
	waitDAVQueue(t, g, 1)
	select {
	case <-entered:
		t.Fatal("same path bodies overlapped")
	default:
	}
	endFirst()
	awaitDAV(t, entered)
	endSecond()
	if <-first != 201 || <-second != 201 {
		t.Fatal("PUT failed")
	}
	b, e := os.ReadFile(filepath.Join(root, "same.zst"))
	if e != nil || string(b) != "second" {
		t.Fatal("publication order", e, string(b))
	}
}

func TestDAVStandardLockTokensSurviveParallelGate(t *testing.T) {
	s, _ := davTestServer(t)
	res := davReq(t, s, "LOCK", "/locked.zst", strings.NewReader(`<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype><D:owner>test</D:owner></D:lockinfo>`), map[string]string{"Depth": "0", "Timeout": "Second-60"})
	token := res.Header.Get("Lock-Token")
	expectStatus(t, res, 201)
	if token == "" {
		t.Fatal("missing standard lock token")
	}
	expectStatus(t, davReq(t, s, "PUT", "/locked.zst", strings.NewReader("rejected"), nil), 423)
	expectStatus(t, davReq(t, s, "PUT", "/unrelated.zst", strings.NewReader("allowed"), nil), 201)
	expectStatus(t, davReq(t, s, "PUT", "/locked.zst", strings.NewReader("with-token"), map[string]string{"If": "(" + token + ")"}), 201)
	expectStatus(t, davReq(t, s, "UNLOCK", "/locked.zst", nil, map[string]string{"Lock-Token": token}), 204)
	if got := string(expectStatus(t, davReq(t, s, "GET", "/locked.zst", nil, nil), 200)); got != "with-token" {
		t.Fatal(got)
	}
}
