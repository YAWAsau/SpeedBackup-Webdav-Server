package sbserver

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Power credentials are separate from DAV and administrator credentials.
// Accepted IDs are durable tombstones: a retry must never schedule a second shutdown.
type powerConfig struct {
	Enabled        bool                   `json:"enabled"`
	TokenHash      string                 `json:"token_hash"`
	Theme          map[string]string      `json:"theme"`
	Seen           map[string]bool        `json:"seen"`
	PairingEnabled bool                   `json:"pairing_enabled"`
	Devices        map[string]powerDevice `json:"devices,omitempty"`
	Owners         map[string]string      `json:"owners,omitempty"`
}
type powerJob struct {
	ID          string `json:"job_id"`
	State       string `json:"state"`
	Preview     bool   `json:"preview"`
	Deadline    int64  `json:"deadline_ms"`
	Message     string `json:"message"`
	Outcome     string `json:"outcome,omitempty"`
	Operation   string `json:"operation,omitempty"`
	created     time.Time
	heartbeat   time.Time
	deferUntil  time.Time
	desktopHash string
	pairedValid func() bool
}
type powerManager struct {
	devicePrompt powerDevicePrompt
	mu           sync.Mutex
	store        *Store
	config       powerConfig
	job          *powerJob
	active       int
	closed       bool
	timer        *time.Timer
	generation   uint64
	err          error
	shutdown     func() error
	launch       func(string) error
	supported    bool
	grants       map[string]powerGrant
	pairEpoch    uint64
	challenges   map[string]powerChallenge
}

func newPowerManager(store *Store) *powerManager {
	p := &powerManager{store: store, shutdown: normalPowerOff, launch: launchPowerWindow, supported: powerSupported()}
	dir := store.internal("power")
	if file, e := latestVersionFile(dir); e == nil {
		b, e := os.ReadFile(file)
		if e == nil {
			e = json.Unmarshal(b, &p.config)
		}
		p.err = e
	} else if !errors.Is(e, os.ErrNotExist) {
		p.err = e
	}
	if p.config.Seen == nil {
		p.config.Seen = map[string]bool{}
	}
	if p.config.Owners == nil {
		p.config.Owners = map[string]string{}
	}
	p.grants = map[string]powerGrant{}
	p.challenges = map[string]powerChallenge{}
	if p.config.Devices == nil {
		p.config.Devices = map[string]powerDevice{}
	}
	return p
}
func (p *powerManager) save() (err error) {
	defer func() {
		if err != nil {
			p.err = err
			p.config.Enabled = false
		}
	}()
	dir := p.store.internal("power")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	n, e := nextVersion(dir)
	if e != nil {
		return e
	}
	return writeImmutableJSON(filepath.Join(dir, fmt.Sprintf("%020d.json", n)), p.config)
}
func (p *powerManager) pending() bool {
	return p.job != nil && (p.job.State == "waiting" || p.job.State == "countdown" || p.job.State == "deferred")
}
func (p *powerManager) finish(state, message string) {
	p.job.State = state
	p.job.Message = message
	p.job.Deadline = 0
	_ = p.store.AppendEvent(AuditEvent{Unix: nowUnix(), Event: "power." + state, Message: message})
}
func (p *powerManager) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	if p.timer != nil {
		p.timer.Stop()
	}
	if p.pending() {
		p.finish("cancelled", "服務停止，已取消關機")
	}
}
func (p *powerManager) arm() {
	generation := p.generation
	p.timer = time.AfterFunc(time.Second, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.closed || generation != p.generation {
			return
		}
		p.tick(time.Now())
		if p.pending() {
			p.arm()
		}
	})
}
func (p *powerManager) tick(now time.Time) {
	if !p.pending() {
		return
	}
	j := p.job
	if j.pairedValid != nil && !j.pairedValid() {
		p.finish("cancelled", "登入或配對授權已失效，已取消關機")
		return
	}
	if now.Sub(j.created) > time.Hour {
		p.finish("cancelled", "請求已逾時，請重新確認備份狀態")
		return
	}
	if !p.config.Enabled && !j.Preview {
		p.finish("cancelled", "備份後關機已停用")
		return
	}
	if p.active > 0 || now.Sub(j.heartbeat) > 10*time.Second {
		j.State = "waiting"
		j.Deadline = 0
		if p.active > 0 {
			j.Message = "等待其他傳輸完成"
		} else {
			j.Message = "等待桌面提示連線；倒數尚未開始"
		}
		return
	}
	if now.Before(j.deferUntil) {
		j.State = "deferred"
		j.Deadline = 0
		j.Message = "已延後 10 分鐘"
		return
	}
	if j.Deadline == 0 {
		j.State = "countdown"
		j.Deadline = now.Add(60 * time.Second).UnixMilli()
		j.Message = "請儲存工作，電腦即將正常關機"
		return
	}
	if now.UnixMilli() < j.Deadline {
		return
	}
	if j.Preview {
		p.finish("simulated", "預覽完成；沒有執行關機")
		return
	}
	// The same mutex guards admission of all DAV/data requests and the final idle check.
	p.finish("shutting_down", "已向 Windows 請求正常關機")
	if e := p.shutdown(); e != nil {
		p.finish("error", "Windows 拒絕關機；請檢查服務權限")
	}
}
func (p *powerManager) begin() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job != nil && p.job.State == "shutting_down" {
		return false
	}
	p.active++
	if p.pending() {
		p.job.Deadline = 0
		p.job.State = "waiting"
		p.job.Message = "等待其他傳輸完成"
	}
	return true
}
func (p *powerManager) end() { p.mu.Lock(); p.active--; p.mu.Unlock() }
func powerToken(r *http.Request, hash string) bool {
	v := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return hash != "" && len(v) < 256 && subtle.ConstantTimeCompare([]byte(SHA256Bytes([]byte(v))), []byte(hash)) == 1
}
func (p *powerManager) view() map[string]any {
	return map[string]any{"enabled": p.config.Enabled, "supported": p.supported, "configured": p.config.TokenHash != "" || p.config.PairingEnabled, "pairing_enabled": p.config.PairingEnabled, "devices": p.config.Devices, "theme": p.config.Theme, "job": p.job, "active_requests": p.active, "ready": p.err == nil, "server_ms": time.Now().UnixMilli()}
}
func (p *powerManager) start(id string, preview bool, listen string) error {
	if p.closed || p.err != nil {
		return errors.New("power state unavailable")
	}
	if p.pending() || p.job != nil && p.job.State == "shutting_down" {
		return errors.New("another power request exists")
	}
	token, e := GenerateToken()
	if e != nil {
		return e
	}
	_, port, e := net.SplitHostPort(listen)
	if e != nil {
		return e
	}
	{
		if len(p.config.Seen) >= 10000 {
			return errors.New("power request history full; administrator maintenance required")
		}
		p.config.Seen[id] = true
		if e = p.save(); e != nil {
			p.err = e
			return e
		}
	}
	p.generation++
	if p.timer != nil {
		p.timer.Stop()
	}
	p.job = &powerJob{ID: id, State: "waiting", Preview: preview, created: time.Now(), desktopHash: SHA256Bytes([]byte(token)), Message: "等待桌面提示連線"}
	// The fragment is never sent as an HTTP URL or stored in request logs.
	if e = p.launch("http://127.0.0.1:" + port + "/api/v1/power/desktop#" + token); e != nil {
		p.finish("error", "無法顯示桌面提示；未安排關機。請登入 Windows 並確認原生通知元件已完整安裝")
	} else {
		p.arm()
	}
	return nil
}
func (s *HTTPServer) powerComplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	p := s.power
	p.mu.Lock()
	defer p.mu.Unlock()
	grant, paired := p.grant(r)
	if !powerToken(r, p.config.TokenHash) && !paired {
		writeErr(w, 401, "shutdown credential required")
		return
	}
	if r.Method == "GET" {
		id := r.PathValue("id")
		if paired && (grant.JobID != id || p.config.Seen[id] && p.config.Owners[id] != grant.DeviceID) {
			writeErr(w, 403, "job scope mismatch")
			return
		}
		if p.job != nil && p.job.ID == id {
			writeJSON(w, 200, p.job)
		} else if p.config.Seen[id] {
			writeJSON(w, 200, map[string]string{"job_id": id, "state": "closed", "message": "Previously accepted; never rescheduled after restart or replacement"})
		} else {
			writeErr(w, 404, "unknown job")
		}
		return
	}
	var in struct {
		JobID     string `json:"job_id"`
		Result    string `json:"result"`
		Outcome   string `json:"outcome"`
		Operation string `json:"operation"`
		Confirmed bool   `json:"user_confirmed"`
		Preview   bool   `json:"preview"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	manual := r.URL.Path == "/api/v1/power/manual"
	valid := decodePickerJSON(r, &in) && ValidateIdentifier(in.JobID, "job_id") == nil && len(in.JobID) >= 16
	if manual {
		valid = valid && in.Confirmed && in.Result == "" && (in.Operation == "backup" || in.Operation == "restore") && (in.Outcome == "success" || in.Outcome == "failed" || in.Outcome == "partial" || in.Outcome == "cancelled")
	} else {
		valid = valid && in.Result == "success" && in.Outcome == "" && in.Operation == "" && !in.Confirmed && !in.Preview
	}
	if !valid {
		writeErr(w, 400, "invalid completion or explicitly confirmed manual shutdown request")
		return
	}
	if paired && (!manual || grant.JobID != in.JobID || p.config.Seen[in.JobID] && p.config.Owners[in.JobID] != grant.DeviceID) {
		writeErr(w, 403, "job scope mismatch")
		return
	}
	if p.config.Seen[in.JobID] {
		if p.job != nil && p.job.ID == in.JobID {
			writeJSON(w, 200, p.job)
		} else {
			writeJSON(w, 200, map[string]string{"job_id": in.JobID, "state": "closed"})
		}
		return
	}
	if !p.supported || (!p.config.Enabled && !in.Preview) {
		writeErr(w, 409, "backup shutdown disabled or unsupported")
		return
	}
	if paired {
		p.config.Owners[in.JobID] = grant.DeviceID
	}
	if e := p.start(in.JobID, in.Preview, s.listen); e != nil {
		if !p.config.Seen[in.JobID] {
			delete(p.config.Owners, in.JobID)
		}
		writeErr(w, 409, e.Error())
		return
	}
	if paired {
		p.job.pairedValid = func() bool { return p.validGrant(grant) }
	}
	if manual {
		p.job.Outcome = in.Outcome
		p.job.Operation = in.Operation
	}
	writeJSON(w, 202, p.job)
}
func validPowerTheme(v map[string]string) bool {
	choices := map[string]string{"theme": "|system|dark|black|white|", "accent": "|blue|teal|violet|", "surface": "|classic|glass|", "density": "|comfortable|compact|", "motion": "|system|reduce|"}
	if len(v) != len(choices) {
		return false
	}
	for k, list := range choices {
		if v[k] == "" || !strings.Contains(list, "|"+v[k]+"|") {
			return false
		}
	}
	return true
}
func (s *HTTPServer) powerAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuth.validSession(r) {
		writeErr(w, 401, "administrator login required")
		return
	}
	p := s.power
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.Method == "GET" {
		writeJSON(w, 200, p.view())
		return
	}
	var in struct {
		Action   string            `json:"action"`
		DeviceID string            `json:"device_id"`
		JobID    string            `json:"job_id"`
		Theme    map[string]string `json:"theme"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if !decodePickerJSON(r, &in) {
		writeErr(w, 400, "invalid power settings")
		return
	}
	var token string
	var e error
	switch in.Action {
	case "enable_pairing", "disable_pairing", "approve_device", "revoke_device", "delete_device", "revoke_pairs":
		e = p.deviceAction(in.Action, in.DeviceID)
	case "enable":
		if !p.supported || (p.config.TokenHash == "" && !p.config.PairingEnabled) || p.err != nil {
			writeErr(w, 409, "create a shutdown credential on a supported Windows host first")
			return
		}
		p.config.Enabled = true
		e = p.save()
	case "disable":
		p.config.Enabled = false
		if p.pending() {
			p.finish("cancelled", "管理員已停用關機")
		}
		e = p.save()
	case "rotate":
		token, e = GenerateToken()
		if e == nil {
			p.config.TokenHash = SHA256Bytes([]byte(token))
			p.config.Enabled = false
			if p.pending() {
				p.finish("cancelled", "授權已更新")
			}
			e = p.save()
		}
	case "theme":
		if !validPowerTheme(in.Theme) {
			writeErr(w, 400, "invalid theme")
			return
		}
		if !reflect.DeepEqual(p.config.Theme, in.Theme) {
			p.config.Theme = in.Theme
			e = p.save()
		}
	case "preview":
		if !p.supported {
			writeErr(w, 409, "Windows desktop required")
			return
		}
		id, _ := GenerateToken()
		e = p.start("preview-"+id, true, s.listen)
	default:
		e = p.action(in.Action, in.JobID)
	}
	if e != nil {
		writeErr(w, 409, e.Error())
		return
	}
	v := p.view()
	if token != "" {
		v["token"] = token
	}
	writeJSON(w, 200, v)
}
func (p *powerManager) action(action, id string) error {
	if !p.pending() || p.job.ID != id {
		return errors.New("request no longer pending")
	}
	switch action {
	case "pause":
		p.job.heartbeat = time.Time{}
		p.job.Deadline = 0
		p.job.State = "waiting"
		p.job.Message = "桌面提示已暫停"
	case "cancel":
		p.finish("cancelled", "使用者已取消關機")
	case "defer":
		p.job.deferUntil = time.Now().Add(10 * time.Minute)
		p.job.Deadline = 0
		p.job.State = "deferred"
		p.job.Message = "已延後 10 分鐘"
	case "now":
		if p.active > 0 || time.Since(p.job.heartbeat) > 10*time.Second {
			return errors.New("transfer active or desktop unavailable")
		}
		p.job.deferUntil = time.Time{}
		p.job.Deadline = time.Now().UnixMilli()
		p.tick(time.Now())
	default:
		return errors.New("unknown power action")
	}
	return nil
}
func (s *HTTPServer) powerDesktop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !pickerLocal(r) || !browserWrite(r) {
		writeErr(w, 403, "local desktop required")
		return
	}
	p := s.power
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == nil || !powerToken(r, p.job.desktopHash) {
		writeErr(w, 401, "desktop capability expired")
		return
	}
	var in struct {
		Action string `json:"action"`
		JobID  string `json:"job_id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if !decodePickerJSON(r, &in) {
		writeErr(w, 400, "invalid desktop request")
		return
	}
	if in.Action == "heartbeat" {
		if time.Since(p.job.heartbeat) > 10*time.Second {
			p.job.Deadline = 0
		}
		p.job.heartbeat = time.Now()
	} else if e := p.action(in.Action, in.JobID); e != nil {
		writeErr(w, 409, e.Error())
		return
	}
	v := p.view()
	delete(v, "devices") // Desktop heartbeat must stay bounded regardless of paired-device inventory.
	writeJSON(w, 200, v)
}
