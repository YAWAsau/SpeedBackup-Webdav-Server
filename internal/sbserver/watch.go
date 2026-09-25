package sbserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Watch connects to the existing authenticated API; it never opens a Store or
// starts/stops a server. It also works with the webdav11 server.
type WatchOptions struct {
	URL, Username, Password, Token string
	Interval                       time.Duration
	Once                           bool
}
type watchSnapshot struct {
	Transfers []davTransfer     `json:"transfers"`
	Summary   davActivityTotals `json:"summary"`
	Millis    int64             `json:"snapshot_ms"`
	Scope     string            `json:"stats_scope"`
	Revision  string            `json:"revision"`
}

func WatchURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("--url must be a server origin, e.g. http://127.0.0.1:8765 (no credentials or path)")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback()))) {
		return "", fmt.Errorf("use HTTPS for remote monitoring, or an SSH tunnel to localhost")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// draw receives trusted layout and sanitized labels, never raw response bodies.
func Watch(ctx context.Context, o WatchOptions, draw func([]string) error) error {
	base, err := WatchURL(o.URL)
	if err != nil {
		return err
	}
	if o.Interval < 250*time.Millisecond || o.Interval > 60*time.Second {
		return fmt.Errorf("--interval must be between 250ms and 1m")
	}
	jar, _ := cookiejar.New(nil)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Local monitoring should never send credentials through a configured proxy.
	u, _ := url.Parse(base)
	if u.Scheme == "http" {
		transport.Proxy = nil
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Jar: jar, Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(c context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
		r, e := http.NewRequestWithContext(c, method, base+endpoint, body)
		if e != nil {
			return nil, e
		}
		if o.Token != "" {
			r.Header.Set("Authorization", "Bearer "+o.Token)
		}
		if method == "POST" {
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-SB-Admin", "1")
		}
		return client.Do(r)
	}
	if o.Token == "" {
		payload, _ := json.Marshal(map[string]string{"username": o.Username, "password": o.Password})
		resp, e := request(ctx, "POST", "/api/v1/auth/login", bytes.NewReader(payload))
		clear(payload)
		o.Password = ""
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("cannot connect to server for login")
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("administrator login failed (HTTP %d); use the WebAdmin account, not a WebDAV share account", resp.StatusCode)
		}
		defer func() {
			c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			r, e := request(c, "POST", "/api/v1/auth/logout", nil)
			if e == nil {
				r.Body.Close()
			}
		}()
	}
	var previous *watchSnapshot
	failures := 0
	for {
		if ctx.Err() != nil {
			return nil
		}
		endpoint := "/api/v1/admin/webdav/activity"
		client.Timeout = 10 * time.Second
		// Wait for changes only when idle; keep --interval as the active cadence.
		// Servers without revisions retain the original polling behavior.
		if previous != nil && previous.Summary.Active == 0 && previous.Revision != "" {
			endpoint += "?watch=1&after=" + url.QueryEscape(previous.Revision)
			client.Timeout = 35 * time.Second
		}
		resp, e := request(ctx, "GET", endpoint, nil)
		var snap watchSnapshot
		if e == nil {
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				resp.Body.Close()
				return fmt.Errorf("monitor session expired or permission denied; run watch again to sign in")
			}
			if resp.StatusCode != 200 {
				e = fmt.Errorf("HTTP %d", resp.StatusCode)
			} else {
				data, re := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
				if re != nil || len(data) > 8*1024*1024 {
					e = fmt.Errorf("invalid monitor response")
				} else {
					e = json.Unmarshal(data, &snap)
				}
				if e == nil && (snap.Millis <= 0 || snap.Scope != "since_server_start") {
					e = fmt.Errorf("server does not provide supported live statistics")
				}
			}
			resp.Body.Close()
		}
		if ctx.Err() != nil {
			return nil
		}
		delay := o.Interval
		if e != nil {
			previous = nil
			failures++
			if o.Once {
				return fmt.Errorf("cannot read live statistics; check server availability and version")
			}
			delay = time.Duration(min(failures, 10)) * time.Second
			if err = draw([]string{"SpeedBackup 即時監看", "連線中斷／資料不可用，速度暫停計算。", fmt.Sprintf("%s  %s 後重試；Ctrl+C 只退出監看。", time.Now().Format("15:04:05"), delay)}); err != nil {
				return err
			}
		} else {
			failures = 0
			if err = draw(watchFrame(snap, previous)); err != nil {
				return err
			}
			previous = &snap
			if o.Once {
				return nil
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func watchBytes(n float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", n, units[i])
}
func watchRate(delta int64, seconds float64) string {
	if seconds <= 0 || delta < 0 {
		return "--"
	}
	return watchBytes(float64(delta)/seconds) + "/s"
}
func safeWatchText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, s)
}
func watchPurpose(v davTransfer) string {
	jsonFile := strings.HasSuffix(strings.ToLower(v.Path), ".json") || strings.Contains(strings.ToLower(v.Path), ".json.part")
	switch v.Operation {
	case "PUT":
		if jsonFile {
			return "更新 JSON 列表"
		}
		return "備份數據上傳"
	case "GET":
		if jsonFile {
			return "獲取應用資訊"
		}
		return "恢復數據下載"
	case "MOVE":
		return "移動／重新命名"
	case "COPY":
		return "複製"
	case "DELETE":
		return "刪除"
	case "MKCOL":
		return "建立目錄"
	}
	return safeWatchText(v.Operation)
}
func watchFrame(s watchSnapshot, prev *watchSnapshot) []string {
	seconds := 0.0
	var upload, download int64
	old := map[uint64]davTransfer{}
	if prev != nil && s.Millis > prev.Millis && s.Summary.UploadBytes >= prev.Summary.UploadBytes && s.Summary.DownloadBytes >= prev.Summary.DownloadBytes {
		seconds = float64(s.Millis-prev.Millis) / 1000
		upload = s.Summary.UploadBytes - prev.Summary.UploadBytes
		download = s.Summary.DownloadBytes - prev.Summary.DownloadBytes
		for _, v := range prev.Transfers {
			old[v.ID] = v
		}
	}
	lines := []string{fmt.Sprintf("SpeedBackup 即時監看  %s  Ctrl+C 退出", time.UnixMilli(s.Millis).Local().Format("2006-01-02 15:04:05")),
		fmt.Sprintf("上傳 %s  %s | 下載 %s  %s", watchBytes(float64(s.Summary.UploadBytes)), watchRate(upload, seconds), watchBytes(float64(s.Summary.DownloadBytes)), watchRate(download, seconds)),
		fmt.Sprintf("進行中 %d | 完成 %d | 失敗 %d", s.Summary.Active, s.Summary.Completed, s.Summary.Failed),
		"累計自服務啟動；下載僅代表伺服器送出，不代表手機恢復完成。", "", "正在傳輸／操作："}
	active := []davTransfer{}
	recent := []davTransfer{}
	for _, v := range s.Transfers {
		if v.Ended == 0 {
			active = append(active, v)
		} else {
			recent = append(recent, v)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].ID < active[j].ID })
	for _, v := range active {
		speed := "--"
		eta := "--"
		progress := "大小未知"
		if p, ok := old[v.ID]; ok && p.Started == v.Started && v.Bytes >= p.Bytes && seconds > 0 {
			speed = watchRate(v.Bytes-p.Bytes, seconds)
			rate := float64(v.Bytes-p.Bytes) / seconds
			if rate > 0 && v.Expected > v.Bytes {
				eta = (time.Duration(math.Min(float64(v.Expected-v.Bytes)/rate, 31536000)) * time.Second).String()
			}
		}
		if v.Expected > 0 {
			progress = fmt.Sprintf("%.1f%% / %s", math.Min(99.9, 100*float64(v.Bytes)/float64(v.Expected)), watchBytes(float64(v.Expected)))
		}
		if !davTransfersData(v.Operation) {
			progress = "處理中"
			speed = "--"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s  %s  %s  ETA %s", watchPurpose(v), watchBytes(float64(v.Bytes)), progress, speed, eta), "  "+safeWatchText(v.Username)+" / "+safeWatchText(v.Path))
		if len(active) > 0 && len(lines) >= 38 {
			lines = append(lines, "  更多進行中項目請查看管理頁。")
			break
		}
	}
	if len(active) == 0 {
		lines = append(lines, "  等待傳輸…")
	}
	lines = append(lines, "", "近期結果：")
	sort.Slice(recent, func(i, j int) bool { return recent[i].Ended > recent[j].Ended })
	for i, v := range recent {
		if i == 5 {
			break
		}
		state := "完成"
		if v.State == "failed" {
			state = "失敗"
		}
		detail := v.Path
		if v.Destination != "" {
			detail += " → " + v.Destination
		}
		if v.Error != "" {
			detail += " (" + v.Error + ")"
		}
		lines = append(lines, fmt.Sprintf("%s %s [%s] %s", time.UnixMilli(v.Ended).Local().Format("15:04:05.000"), state, watchPurpose(v), safeWatchText(detail)))
	}
	return lines
}
