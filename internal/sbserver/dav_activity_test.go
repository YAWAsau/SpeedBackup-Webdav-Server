package sbserver

import (
	"context"
	"testing"
	"time"
)

func TestDAVLifecycleDoesNotWaitForProgressSample(t *testing.T) {
	a := &davActivity{}
	v := a.start("phone", "PUT", "b/app/file", -1)
	// A dirty revision must return before even a very long progress interval.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := a.wait(ctx, 0, time.Hour, time.Hour); err != nil {
		t.Fatal("start was delayed", err)
	}
	_, rev := a.snapshotRevision()
	done := make(chan error, 1)
	go func() { done <- a.wait(ctx, rev, time.Hour, time.Hour) }()
	a.add(v, 15)
	a.finish(v, 201, nil)
	if err := <-done; err != nil {
		t.Fatal("finish was delayed", err)
	}
}

func TestDAVTotalsOutliveRecentHistory(t *testing.T) {
	a := &davActivity{}
	for i := 0; i < 205; i++ {
		v := a.start("phone", "PUT", "backup/file", -1)
		a.add(v, 10)
		a.finish(v, 201, nil)
	}
	live := a.start("phone", "GET", "backup/file", 100)
	a.add(live, 30)
	rows, _, totals := a.snapshotState()
	if len(rows) != 201 || totals.Active != 1 || totals.Completed != 205 || totals.UploadBytes != 2050 || totals.DownloadBytes != 30 {
		t.Fatalf("%d %+v", len(rows), totals)
	}
	a.finish(live, 200, nil) // A truncated known length is a failure.
	rows, _, totals = a.snapshotState()
	if len(rows) != 200 || totals.Active != 0 || totals.Completed != 205 || totals.Failed != 1 || totals.DownloadBytes != 30 {
		t.Fatalf("%d %+v", len(rows), totals)
	}
	if rows[1].Expected != -1 || rows[1].Bytes != 10 || rows[1].State != "transfer_complete" {
		t.Fatal("unknown length must remain truthful", rows[1])
	}
}
