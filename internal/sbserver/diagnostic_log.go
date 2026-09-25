package sbserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const diagnosticFileLimit = 8 << 20
const diagnosticCopies = 4 // current plus three older files
const diagnosticQueueLimit = 2 << 20

// A short-lived timer batches request records. Streaming chunks never wait for
// disk IO; queue pressure and write failures remain visible in the debug bundle.
type diagnosticLog struct {
	mu           sync.Mutex
	writeMu      sync.Mutex
	filename     string
	limit        int64
	pending      [][]byte
	pendingBytes int
	timer        *time.Timer
	dropped      uint64
	writeErrors  uint64
	lastError    string
}

func (l *diagnosticLog) append(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	b = append(b, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pendingBytes+len(b) > diagnosticQueueLimit {
		l.dropped++
		return
	}
	l.pending = append(l.pending, b)
	l.pendingBytes += len(b)
	if l.timer == nil {
		l.timer = time.AfterFunc(200*time.Millisecond, func() { _ = l.flush() })
	}
}

func rotateLog(name string, limit int64, incoming int64) error {
	info, err := os.Stat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size()+incoming <= limit || info.Size() == 0 {
		return nil
	}
	if err = os.Remove(fmt.Sprintf("%s.%d", name, diagnosticCopies-1)); err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := diagnosticCopies - 2; i >= 0; i-- {
		old := name
		if i > 0 {
			old = fmt.Sprintf("%s.%d", name, i)
		}
		if err = os.Rename(old, fmt.Sprintf("%s.%d", name, i+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (l *diagnosticLog) flush() error {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	l.mu.Lock()
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	batch := l.pending
	l.pending = nil
	l.pendingBytes = 0
	l.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	err := os.MkdirAll(filepath.Dir(l.filename), 0700)
	var written int
	if err == nil {
		size := 0
		for _, b := range batch {
			size += len(b)
		}
		err = rotateLog(l.filename, l.limit, int64(size))
	}
	if err == nil {
		var f *os.File
		f, err = os.OpenFile(l.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			for _, b := range batch {
				if _, err = f.Write(b); err != nil {
					break
				}
				written++
			}
			if err == nil {
				err = f.Sync()
			}
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		l.mu.Lock()
		l.writeErrors++
		l.dropped += uint64(len(batch) - written)
		l.lastError = err.Error()
		l.mu.Unlock()
	}
	return err
}

func (l *diagnosticLog) status() map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	return map[string]any{"pending_records": len(l.pending), "dropped_records": l.dropped, "write_errors": l.writeErrors, "last_write_error": l.lastError, "file_limit_bytes": l.limit, "retained_files": diagnosticCopies, "flush_interval_ms": 200}
}

type diagnosticWriter struct{ log *diagnosticLog }

func (w diagnosticWriter) Write(b []byte) (int, error) {
	w.log.append(map[string]any{"time": time.Now().Format(time.RFC3339Nano), "kind": "http_server_error", "message": clipDiagnostic(string(b), 4096)})
	return len(b), nil
}

func clipDiagnostic(s string, limit int) string {
	if len(s) > limit {
		return s[:limit] + " [truncated]"
	}
	return s
}
