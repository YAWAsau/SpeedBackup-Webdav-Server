package sbserver

import (
	"context"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// These are transport observations, not Android installation/extraction facts.
type davTransfer struct {
	RequestID   string `json:"request_id,omitempty"`
	ID          uint64 `json:"id"`
	Username    string `json:"username"`
	Operation   string `json:"operation"`
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
	Backup      string `json:"backup"`
	App         string `json:"app"`
	File        string `json:"file"`
	Bytes       int64  `json:"bytes"`
	Expected    int64  `json:"expected"`
	Started     int64  `json:"started_ms"`
	Ended       int64  `json:"ended_ms"`
	Status      int    `json:"http_status"`
	State       string `json:"state"`
	Error       string `json:"error,omitempty"`
}
type davActivity struct {
	mu       sync.Mutex
	next     uint64
	active   map[uint64]*davLiveTransfer
	recent   []davTransfer
	revision uint64
	changed  chan struct{}
	totals   davActivityTotals
}

type davActivityTotals struct {
	Active        int    `json:"active"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
	Completed     uint64 `json:"completed"`
	Failed        uint64 `json:"failed"`
	LastEnded     int64  `json:"last_ended_ms"`
}

// Bytes are local to each transfer: streaming never takes the activity mutex.
// Immutable metadata is copied to snapshots, not the atomic values themselves.
type davLiveTransfer struct {
	meta     davTransfer
	bytes    atomic.Int64
	expected atomic.Int64
}

func (v *davLiveTransfer) snapshot() davTransfer {
	out := v.meta
	out.Bytes = v.bytes.Load()
	out.Expected = v.expected.Load()
	return out
}
func (a *davActivity) signalLocked() {
	a.revision++
	if a.changed != nil {
		close(a.changed)
	}
	a.changed = make(chan struct{})
}
func davTransfersData(method string) bool { return method == "GET" || method == "PUT" }

func (a *davActivity) start(user, method, name string, expected int64, destination ...string) *davLiveTransfer {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active == nil {
		a.active = map[uint64]*davLiveTransfer{}
	}
	a.next++
	clean := strings.TrimPrefix(path.Clean("/"+name), "/")
	parts := strings.Split(clean, "/")
	v := &davLiveTransfer{meta: davTransfer{ID: a.next, Username: user, Operation: method, Path: clean, File: path.Base(clean), Started: time.Now().UnixMilli(), State: "transferring"}}
	if !davTransfersData(method) {
		v.meta.State = "processing"
	}
	if len(destination) > 0 {
		v.meta.Destination = destination[0]
	}
	if len(destination) > 1 {
		v.meta.RequestID = destination[1]
	}
	v.expected.Store(expected)
	if len(parts) > 1 {
		v.meta.Backup = parts[0]
	}
	if len(parts) > 2 {
		v.meta.App = parts[len(parts)-2]
	}
	a.active[v.meta.ID] = v
	a.signalLocked()
	return v
}
func (a *davActivity) add(v *davLiveTransfer, n int)      { v.bytes.Add(int64(n)) }
func (a *davActivity) expect(v *davLiveTransfer, n int64) { v.expected.Store(n) }
func (a *davActivity) finish(live *davLiveTransfer, status int, err error) davTransfer {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := live.snapshot()
	v.Ended = time.Now().UnixMilli()
	v.Status = status
	v.State = "transfer_complete"
	operation := !davTransfersData(v.Operation)
	if operation {
		v.State = "operation_complete"
	}
	if err != nil || status < 200 || status >= 300 || (operation && status == 207) || (!operation && v.Expected >= 0 && v.Bytes != v.Expected) {
		v.State = "failed"
		if err != nil {
			v.Error = err.Error()
		} else if operation && status == 207 {
			v.Error = "operation returned multi-status; completion not confirmed"
		} else if status >= 200 && status < 300 {
			v.Error = "transferred byte count differs from response length"
		}
	}
	delete(a.active, v.ID)
	if v.Operation == "PUT" {
		a.totals.UploadBytes += v.Bytes
	} else if v.Operation == "GET" {
		a.totals.DownloadBytes += v.Bytes
	}
	if v.State == "failed" {
		a.totals.Failed++
	} else {
		a.totals.Completed++
	}
	a.totals.LastEnded = v.Ended
	a.recent = append(a.recent, v)
	if len(a.recent) > 200 {
		a.recent = append([]davTransfer(nil), a.recent[len(a.recent)-200:]...)
	}
	a.signalLocked()
	return v
}
func (a *davActivity) snapshot() []davTransfer {
	out, _ := a.snapshotRevision()
	return out
}
func (a *davActivity) snapshotRevision() ([]davTransfer, uint64) {
	out, revision, _ := a.snapshotState()
	return out, revision
}
func (a *davActivity) snapshotState() ([]davTransfer, uint64, davActivityTotals) {
	a.mu.Lock()
	totals := a.totals
	totals.Active = len(a.active)
	out := append([]davTransfer{}, a.recent...)
	for _, v := range a.active {
		transfer := v.snapshot()
		out = append(out, transfer)
		if transfer.Operation == "PUT" {
			totals.UploadBytes += transfer.Bytes
		} else if transfer.Operation == "GET" {
			totals.DownloadBytes += transfer.Bytes
		}
	}
	revision := a.revision
	a.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, revision, totals
}

// Long polling preserves ordinary authenticated HTTP and reverse-proxy support.
// Idle observers sleep until start/finish events or a 25-second heartbeat. Only
// live observers sample progress; no permanent background ticker or per-chunk
// notification queues are created. Bursts are coalesced into one snapshot.
func (a *davActivity) wait(ctx context.Context, after uint64, progressInterval, heartbeat time.Duration) error {
	a.mu.Lock()
	if a.changed == nil {
		a.changed = make(chan struct{})
	}
	changed, dirty, active := a.changed, a.revision != after, len(a.active) > 0
	a.mu.Unlock()
	if dirty {
		return ctx.Err()
	}
	delay := heartbeat
	if active {
		delay = progressInterval
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	case <-changed:
		// Start/finish transitions wake immediately, including while sampling
		// active transfers. Do not lose short transfers to a post-event delay.
		return ctx.Err()
	}
}
func (d *davService) activityHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("watch") == "1" {
		after, err := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
		if err != nil {
			writeErr(w, 400, "invalid activity revision")
			return
		}
		if d.activity.wait(r.Context(), after, 250*time.Millisecond, 25*time.Second) != nil {
			return
		}
	}
	transfers, revision, totals := d.activity.snapshotState()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"transfers": transfers, "revision": strconv.FormatUint(revision, 10), "summary": totals, "snapshot_ms": time.Now().UnixMilli(), "stats_scope": "since_server_start", "scope": "server_transport_only", "client_restore_status": "not_reported", "history_limit": 200})
}

type davReadMeter struct {
	io.ReadCloser
	a   *davActivity
	v   *davLiveTransfer
	err error
}

func (m *davReadMeter) Read(p []byte) (int, error) {
	n, e := m.ReadCloser.Read(p)
	m.a.add(m.v, n)
	if e != nil && e != io.EOF {
		m.err = e
	}
	return n, e
}

type davWriteMeter struct {
	http.ResponseWriter
	a        *davActivity
	v        *davLiveTransfer
	status   int
	err      error
	download bool
}

func (m *davWriteMeter) WriteHeader(code int) {
	if m.status != 0 {
		return
	}
	m.status = code
	if m.download && code >= 200 && code < 300 {
		if n, e := strconv.ParseInt(m.Header().Get("Content-Length"), 10, 64); e == nil {
			m.a.expect(m.v, n)
		}
	}
	m.ResponseWriter.WriteHeader(code)
}
func (m *davWriteMeter) Write(p []byte) (int, error) {
	if m.status == 0 {
		m.WriteHeader(200)
	}
	n, e := m.ResponseWriter.Write(p)
	if m.download && m.status >= 200 && m.status < 300 {
		m.a.add(m.v, n)
	}
	if e != nil {
		m.err = e
	}
	return n, e
}
func (m *davWriteMeter) Unwrap() http.ResponseWriter { return m.ResponseWriter }
func (m *davWriteMeter) Flush() {
	if m.status == 0 {
		m.WriteHeader(200)
	}
	_ = http.NewResponseController(m.ResponseWriter).Flush()
}
