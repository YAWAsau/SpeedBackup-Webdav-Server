package sbserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Tail only the end of the audit file. A viewer never acquires the server root
// lock, creates a Store, or rewrites events. This also works beside a service.
func auditTailOffset(f *os.File, lines int) (int64, error) {
	end, err := f.Seek(0, io.SeekEnd)
	if err != nil || lines == 0 {
		return end, err
	}
	pos := end
	found := 0
	buf := make([]byte, 32*1024)
	for pos > 0 {
		n := int64(len(buf))
		if n > pos {
			n = pos
		}
		pos -= n
		if _, err = f.ReadAt(buf[:n], pos); err != nil {
			return 0, err
		}
		for i := int(n) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				if pos+int64(i) == end-1 {
					continue
				}
				found++
				if found == lines {
					return pos + int64(i) + 1, nil
				}
			}
		}
	}
	return 0, nil
}

func writeAuditLine(out io.Writer, line []byte) error {
	var ev struct {
		Unix    int64           `json:"unix"`
		Event   string          `json:"event"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return nil
	}
	stamp := time.Unix(ev.Unix, 0).Local().Format("2006/01/02 15:04:05")
	if ev.Event == "webdav_transfer" {
		var v davTransfer
		if err := json.Unmarshal(ev.Details, &v); err != nil {
			return nil
		}
		level := "INFO"
		if v.State == "failed" {
			level = "ERROR"
		}
		if v.Started > 0 {
			stamp = time.UnixMilli(v.Started).Local().Format("2006/01/02 15:04:05.000 -07:00")
		}
		_, err := fmt.Fprintf(out, "%s %-5s : %s %q account=%q status=%d bytes=%d elapsed=%.3fs state=%s", stamp, level, v.Operation, v.Path, v.Username, v.Status, v.Bytes, float64(v.Ended-v.Started)/1000, v.State)
		if err != nil {
			return err
		}
		if v.Destination != "" {
			if _, err = fmt.Fprintf(out, " destination=%q", v.Destination); err != nil {
				return err
			}
		}
		if v.Error != "" {
			if _, err = fmt.Fprintf(out, " error=%q", v.Error); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(out)
		return err
	}
	_, err := fmt.Fprintf(out, "%s INFO  : %q %q\n", stamp, ev.Event, ev.Message)
	return err
}

// StreamAuditLog shows completed requests, not speculative Android progress.
// Polling is confined to this optional viewer; server transfer I/O is unchanged.
func StreamAuditLog(ctx context.Context, root string, out io.Writer, lines int, follow bool) error {
	if lines < 0 || lines > 1000 {
		return fmt.Errorf("lines must be 0-1000")
	}
	name := filepath.Join(root, ".speedbackup-server", "audit", "events.jsonl")
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("open server event log (use the service data root): %w", err)
	}
	defer func() { _ = f.Close() }()
	offset, err := auditTailOffset(f, lines)
	if err != nil {
		return err
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(f, 32*1024)
	pending := []byte{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		piece, e := reader.ReadSlice('\n')
		offset += int64(len(piece))
		pending = append(pending, piece...)
		if len(pending) > 4*1024*1024 {
			return fmt.Errorf("audit line exceeds 4 MiB")
		}
		if e == nil {
			if err = writeAuditLine(out, bytes.TrimSpace(pending)); err != nil {
				return err
			}
			pending = pending[:0]
			continue
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		if e != io.EOF {
			return e
		}
		if !follow {
			return nil
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		current, e := os.Stat(name)
		if e != nil {
			return e
		}
		opened, e := f.Stat()
		if e != nil {
			return e
		}
		if current.Size() < offset || !os.SameFile(current, opened) {
			replacement, e := os.Open(name)
			if e != nil {
				return e
			}
			_ = f.Close()
			f = replacement
			reader.Reset(f)
			offset = 0
			pending = nil
		}
	}
}
