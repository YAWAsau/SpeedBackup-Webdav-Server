package sbserver

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTerminalLogTailAndSafeFormatting(t *testing.T) {
	s, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 40; i++ {
		if e = s.AppendEvent(AuditEvent{Unix: 1, Event: "test", Message: "old"}); e != nil {
			t.Fatal(e)
		}
	}
	v := davTransfer{Operation: "MOVE", Path: "備份/app/a\nforged", Destination: "備份/app/b\nforged", Username: "phone", State: "operation_complete", Status: 201, Bytes: 512, Started: 1123, Ended: 1373}
	if e = s.AppendEvent(AuditEvent{Unix: 2, Event: "webdav_transfer", Details: v}); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = StreamAuditLog(context.Background(), s.Root, &out, 1, false); e != nil {
		t.Fatal(e)
	}
	got := out.String()
	if !strings.Contains(got, time.UnixMilli(v.Started).Local().Format("2006/01/02 15:04:05.000 -07:00")) || !strings.Contains(got, `destination="備份/app/b\nforged"`) {
		t.Fatal("missing precise operation timestamp or escaped destination", got)
	}
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, `a\nforged`) || !strings.Contains(got, "bytes=512 elapsed=0.250s") || strings.Contains(got, "old") {
		t.Fatal(got)
	}
	out.Reset()
	if e = StreamAuditLog(context.Background(), s.Root, &out, 0, false); e != nil || out.Len() != 0 {
		t.Fatal(e, out.String())
	}
	if e = StreamAuditLog(context.Background(), s.Root, &out, 1001, false); e == nil {
		t.Fatal("unbounded tail accepted")
	}
}

func TestTerminalLogFollowCompletesPartialLine(t *testing.T) {
	s, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	name := filepath.Join(s.Root, ".speedbackup-server", "audit", "events.jsonl")
	partial := `{"unix":1,"event":"test","message":"par`
	if e = os.WriteFile(name, []byte(partial), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- StreamAuditLog(ctx, s.Root, &out, 20, true) }()
	f, e := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.WriteString("tial\"}\n")
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	// Cancel is bounded even when no more bytes are appended.
	time.Sleep(350 * time.Millisecond)
	cancel()
	select {
	case e = <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("viewer did not cancel")
	}
	if strings.Count(out.String(), "\n") != 1 || !strings.Contains(out.String(), "partial") {
		t.Fatal(out.String())
	}
}
