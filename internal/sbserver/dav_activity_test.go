package sbserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestDAVProgressDeltaPreservesTotalsAndLifecycle(t *testing.T) {
	d := &davService{}
	for i := 0; i < 200; i++ {
		v := d.activity.start("phone", "PUT", "backup/file", -1)
		d.activity.add(v, 10)
		d.activity.finish(v, 201, nil)
	}
	live := d.activity.start("phone", "GET", "backup/live", 1000)
	d.activity.add(live, 100)
	_, rev, _ := d.activity.snapshotState()
	read := func(query string) (map[string]json.RawMessage, int) {
		w := httptest.NewRecorder()
		d.activityHandler(w, httptest.NewRequest("GET", "/activity"+query, nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result, w.Body.Len()
	}
	_, fullBytes := read("")
	delta, deltaBytes := read("?watch=1&delta=1&after=" + strconv.FormatUint(rev, 10))
	var rows []davTransfer
	json.Unmarshal(delta["transfers"], &rows)
	if string(delta["partial"]) != "true" || len(rows) != 1 || rows[0].Bytes != 100 {
		t.Fatal("bad progress delta", string(delta["transfers"]))
	}
	var totals davActivityTotals
	json.Unmarshal(delta["summary"], &totals)
	if totals.Completed != 200 || totals.UploadBytes != 2000 || totals.DownloadBytes != 100 {
		t.Fatal(totals)
	}
	if deltaBytes*10 >= fullBytes {
		t.Fatalf("history not omitted: full=%d delta=%d", fullBytes, deltaBytes)
	}
	d.activity.add(live, 900)
	d.activity.finish(live, 200, nil)
	complete, _ := read("?watch=1&delta=1&after=" + strconv.FormatUint(rev, 10))
	json.Unmarshal(complete["transfers"], &rows)
	if string(complete["partial"]) != "false" || len(rows) != 200 || rows[0].State != "transfer_complete" {
		t.Fatal("completion must resync history")
	}
	legacy, _ := read("?delta=1")
	if string(legacy["partial"]) != "false" {
		t.Fatal("ordinary clients require full snapshot")
	}
	t.Logf("full snapshot %d bytes; progress delta %d bytes", fullBytes, deltaBytes)
}

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
