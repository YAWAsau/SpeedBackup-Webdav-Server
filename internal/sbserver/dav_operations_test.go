package sbserver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDAVOperationHistoryMatchesHTTPResults(t *testing.T) {
	for _, prefix := range []string{"/", "/dav/"} {
		t.Run(prefix, func(t *testing.T) {
			s, root := davTestServer(t)
			started := time.Now().UnixMilli()
			dir := prefix + url.PathEscape("備份") + "/"
			original := dir + "apps.json.part.1789810489254.13"
			final := dir + "apps.json"
			copied := dir + "copy.json"
			expectStatus(t, davReq(t, s, "MKCOL", dir, nil, nil), 201)
			expectStatus(t, davReq(t, s, "PUT", original, strings.NewReader("{}"), nil), 201)
			expectStatus(t, davReq(t, s, "HEAD", original, nil, nil), 200)
			expectStatus(t, davReq(t, s, "PROPFIND", dir, nil, map[string]string{"Depth": "1"}), 207)
			expectStatus(t, davReq(t, s, "MOVE", original, nil, map[string]string{"Destination": s.URL + final}), 201)
			expectStatus(t, davReq(t, s, "COPY", final, nil, map[string]string{"Destination": s.URL + copied}), 201)
			if got := string(expectStatus(t, davReq(t, s, "GET", copied, nil, nil), 200)); got != "{}" {
				t.Fatal(got)
			}
			expectStatus(t, davReq(t, s, "DELETE", copied, nil, nil), 204)
			expectStatus(t, davReq(t, s, "DELETE", copied, nil, nil), 404)
			expectStatus(t, davReq(t, s, "MOVE", final, nil, map[string]string{"Destination": "http://secret:private@example.invalid/target"}), 400)
			body := expectStatus(t, doReq(t, s.Client(), "GET", s.URL+"/api/v1/admin/webdav/activity", "admin", nil, nil), 200)
			var state struct {
				Transfers []davTransfer     `json:"transfers"`
				Totals    davActivityTotals `json:"totals"`
			}
			if err := json.Unmarshal(body, &state); err != nil {
				t.Fatal(err)
			}
			if len(state.Transfers) != 8 {
				t.Fatalf("queries must not flood history: %s", body)
			}
			methods := []string{"MOVE", "DELETE", "DELETE", "GET", "COPY", "MOVE", "PUT", "MKCOL"}
			for i, v := range state.Transfers {
				if v.Operation != methods[i] || v.Started < started || v.Ended < v.Started || v.Ended > time.Now().UnixMilli() {
					t.Fatalf("bad operation/time: %+v", v)
				}
				want := "operation_complete"
				if davTransfersData(v.Operation) {
					want = "transfer_complete"
				}
				if i < 2 {
					want = "failed"
				}
				if v.State != want {
					t.Fatalf("HTTP outcome not preserved: %+v", v)
				}
				if !davTransfersData(v.Operation) && v.Bytes != 0 {
					t.Fatalf("mutation inflated traffic: %+v", v)
				}
			}
			if state.Transfers[0].Destination != "" || state.Transfers[4].Destination != "備份/copy.json" || state.Transfers[5].Destination != "備份/apps.json" {
				t.Fatalf("bad destinations: %s", body)
			}
			// The durable log carries the same operation, millisecond timestamp and
			// destination after short-lived rows leave the in-memory history.
			data, err := os.ReadFile(filepath.Join(root, ".speedbackup-server", "audit", "events.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "private") {
				t.Fatal("rejected destination credentials leaked")
			}
			for _, v := range state.Transfers {
				if !strings.Contains(string(data), fmt.Sprintf(`"started_ms":%d`, v.Started)) {
					t.Fatalf("missing durable timestamp: %+v", v)
				}
			}
		})
	}
}

func TestDAVMutationLifecycleAndTotals(t *testing.T) {
	a := &davActivity{}
	for _, method := range []string{"DELETE", "MOVE", "COPY", "MKCOL"} {
		v := a.start("phone", method, "a", -1, "b")
		rows, _, totals := a.snapshotState()
		if totals.Active != 1 || rows[0].State != "processing" {
			t.Fatal(rows, totals)
		}
		result := a.finish(v, 204, nil)
		if result.State != "operation_complete" || result.Destination != "b" {
			t.Fatal(result)
		}
	}
	v := a.start("phone", "DELETE", "a", -1)
	if result := a.finish(v, 207, nil); result.State != "failed" {
		t.Fatal("multi-status incorrectly declared complete", result)
	}
	_, _, totals := a.snapshotState()
	if totals.Active != 0 || totals.UploadBytes != 0 || totals.DownloadBytes != 0 || totals.Completed != 4 || totals.Failed != 1 {
		t.Fatal(totals)
	}
}
