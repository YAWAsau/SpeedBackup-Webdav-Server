package sbserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// Internal publication coordination, separate from the standard WebDAV lock
// tokens. Hold overlapping paths until staged files have actually been renamed.
// Independent files may stream concurrently; directory operations cover children.
type davMutationGate struct {
	mu      sync.Mutex
	changed chan struct{}
	active  map[*davMutationWaiter]bool
	waiting []*davMutationWaiter
}
type davMutationWaiter struct{ paths []string }

func (g *davMutationGate) acquireNames(ctx context.Context, directory string, names []string) (func(), error) {
	for {
		keys, err := davMutationPaths(directory, names...)
		if err != nil {
			return nil, err
		}
		release, err := g.acquire(ctx, keys)
		if err != nil {
			return nil, err
		}
		// A queued directory operation may have changed a symlink alias. Resolve
		// again after acquiring the lease, and retry if its physical target moved.
		current, err := davMutationPaths(directory, names...)
		if err == nil && slices.Equal(keys, current) {
			return release, nil
		}
		release()
		if err != nil {
			return nil, err
		}
	}
}

func davPathsOverlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y || strings.HasPrefix(x, y+"/") || strings.HasPrefix(y, x+"/") {
				return true
			}
		}
	}
	return false
}
func (g *davMutationGate) notify() { close(g.changed); g.changed = make(chan struct{}) }
func (g *davMutationGate) acquire(ctx context.Context, paths []string) (func(), error) {
	g.mu.Lock()
	if g.changed == nil {
		g.changed = make(chan struct{})
		g.active = map[*davMutationWaiter]bool{}
	}
	w := &davMutationWaiter{paths: paths}
	g.waiting = append(g.waiting, w)
	remove := func() {
		for i, p := range g.waiting {
			if p == w {
				g.waiting = append(g.waiting[:i], g.waiting[i+1:]...)
				return
			}
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			remove()
			g.notify()
			g.mu.Unlock()
			return nil, err
		}
		blocked := false
		for p := range g.active {
			if davPathsOverlap(paths, p.paths) {
				blocked = true
				break
			}
		}
		if !blocked {
			for _, p := range g.waiting {
				if p == w {
					break
				}
				if davPathsOverlap(paths, p.paths) {
					blocked = true
					break
				}
			}
		}
		if !blocked {
			remove()
			g.active[w] = true
			g.mu.Unlock()
			var once sync.Once
			return func() { once.Do(func() { g.mu.Lock(); delete(g.active, w); g.notify(); g.mu.Unlock() }) }, nil
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-changed:
		}
		g.mu.Lock()
	}
}

// Include both the lexical path and resolved filesystem path. This coordinates
// interior symlink aliases and case-insensitive Windows names as well as URLs.
// Resolution is for locking only; os.Root remains the filesystem access boundary.
func davMutationPaths(directory string, names ...string) ([]string, error) {
	key := func(name string) string {
		name = filepath.ToSlash(filepath.Clean(name))
		if runtime.GOOS == "windows" {
			name = strings.ToLower(name)
		}
		return strings.TrimSuffix(name, "/")
	}
	keys := []string{}
	for _, name := range names {
		rel, err := localDAVName(name)
		if err != nil {
			return nil, err
		}
		full := filepath.Join(directory, filepath.FromSlash(rel))
		keys = append(keys, key(full))
		probe := full
		tail := []string{}
		for {
			resolved, err := davResolvePath(probe)
			if err == nil {
				for i := len(tail) - 1; i >= 0; i-- {
					resolved = filepath.Join(resolved, tail[i])
				}
				keys = append(keys, key(resolved))
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			parent := filepath.Dir(probe)
			if parent == probe {
				return nil, err
			}
			tail = append(tail, filepath.Base(probe))
			probe = parent
		}
	}
	return keys, nil
}
