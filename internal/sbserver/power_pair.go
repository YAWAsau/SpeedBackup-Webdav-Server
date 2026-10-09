package sbserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

type powerDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Approved bool   `json:"approved"`
	Revoked  bool   `json:"revoked"`
	Expires  int64  `json:"expires_unix"`
}
type powerChallenge struct {
	Device         powerDevice
	Key            *ecdsa.PublicKey
	JobID, Message string
	Expires        int64
}
type powerDevicePrompt struct {
	ID, Hash string
	Expires  int64
}
type powerGrant struct {
	DeviceID, JobID string
	Expires         int64
	Epoch           uint64
}

func (p *powerManager) validGrant(g powerGrant) bool {
	d, ok := p.config.Devices[g.DeviceID]
	return ok && d.Approved && !d.Revoked && p.config.PairingEnabled && p.err == nil && !p.closed && g.Epoch == p.pairEpoch && time.Now().Unix() < g.Expires
}
func (p *powerManager) grant(r *http.Request) (powerGrant, bool) {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") || len(v) > 256 {
		return powerGrant{}, false
	}
	g, ok := p.grants[SHA256Bytes([]byte(strings.TrimPrefix(v, "Bearer ")))]
	return g, ok && p.validGrant(g)
}
func (p *powerManager) deviceAction(action, id string) error {
	switch action {
	case "enable_pairing":
		if !p.supported || p.err != nil {
			return errors.New("pairing unavailable")
		}
		p.config.PairingEnabled = true
	case "disable_pairing":
		p.config.PairingEnabled = false
		p.pairEpoch++
		clear(p.grants)
		clear(p.challenges)
		p.devicePrompt = powerDevicePrompt{}
	case "revoke_pairs":
		p.pairEpoch++
		clear(p.grants)
		clear(p.challenges)
		p.devicePrompt = powerDevicePrompt{}
	case "delete_device":
		if _, ok := p.config.Devices[id]; !ok {
			return errors.New("unknown device")
		}
		delete(p.config.Devices, id)
		for k, g := range p.grants {
			if g.DeviceID == id {
				delete(p.grants, k)
			}
		}
		for k, c := range p.challenges {
			if c.Device.ID == id {
				delete(p.challenges, k)
			}
		}
		if p.devicePrompt.ID == id {
			p.devicePrompt = powerDevicePrompt{}
		}
	case "approve_device", "revoke_device":
		d, ok := p.config.Devices[id]
		if !ok {
			return errors.New("unknown device")
		}
		if action == "approve_device" {
			if !d.Revoked && !d.Approved && time.Now().Unix() >= d.Expires {
				return errors.New("pairing request expired")
			}
			d.Approved = true
			d.Revoked = false
			d.Expires = 0
		} else {
			d.Approved = false
			d.Revoked = true
			for k, g := range p.grants {
				if g.DeviceID == id {
					delete(p.grants, k)
				}
			}
		}
		p.config.Devices[id] = d
	}
	if p.pending() && p.job.pairedValid != nil && !p.job.pairedValid() {
		p.finish("cancelled", "裝置授權已撤銷")
	}
	return p.save()
}
func (s *HTTPServer) pairReady(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		writeErr(w, 400, "application/json required")
		return false
	}
	p := s.power
	if !p.supported || p.err != nil || p.closed {
		writeErr(w, 409, "pairing unavailable")
		return false
	}
	if !p.config.PairingEnabled {
		writeErr(w, 403, "pairing disabled")
		return false
	}
	return true
}
func (s *HTTPServer) powerChallenge(w http.ResponseWriter, r *http.Request) {
	p := s.power
	p.mu.Lock()
	defer p.mu.Unlock()
	if !s.pairReady(w, r) {
		return
	}
	var in struct {
		PublicKey string `json:"public_key"`
		Name      string `json:"device_name"`
		JobID     string `json:"job_id"`
		Scope     string `json:"scope"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if !decodePickerJSON(r, &in) || in.Scope != "power:manual" || len(in.JobID) < 16 || ValidateIdentifier(in.JobID, "job_id") != nil || len(in.Name) > 96 {
		writeErr(w, 400, "invalid pairing request")
		return
	}
	der, e := base64.RawURLEncoding.DecodeString(in.PublicKey)
	if e != nil {
		writeErr(w, 400, "invalid public key")
		return
	}
	parsed, e := x509.ParsePKIXPublicKey(der)
	key, ok := parsed.(*ecdsa.PublicKey)
	if e != nil || !ok || key.Curve != elliptic.P256() {
		writeErr(w, 400, "P256 public key required")
		return
	}
	id := SHA256Bytes(der)
	if d, ok := p.config.Devices[id]; ok && d.Revoked {
		writeErr(w, 403, "device revoked")
		return
	}
	if p.config.Seen[in.JobID] && p.config.Owners[in.JobID] != id {
		writeErr(w, 403, "job owner mismatch")
		return
	}
	now := time.Now().Unix()
	for k, c := range p.challenges {
		if c.Expires <= now {
			delete(p.challenges, k)
		}
	}
	if len(p.challenges) >= 512 {
		writeErr(w, 429, "challenge capacity")
		return
	}
	nonce, e := GenerateToken()
	if e != nil {
		writeErr(w, 409, "random source unavailable")
		return
	}
	message := "SpeedBackup power device v1\n" + nonce + "\n" + id + "\n" + in.JobID + "\npower:manual"
	p.challenges[nonce] = powerChallenge{Device: powerDevice{ID: id, Name: in.Name, Expires: now + 600}, Key: key, JobID: in.JobID, Message: message, Expires: now + 120}
	writeJSON(w, 200, map[string]any{"challenge_id": nonce, "message": message, "expires_unix": now + 120, "device_id": id})
}
func (s *HTTPServer) powerPair(w http.ResponseWriter, r *http.Request) {
	p := s.power
	p.mu.Lock()
	defer p.mu.Unlock()
	if !s.pairReady(w, r) {
		return
	}
	var in struct {
		ChallengeID string `json:"challenge_id"`
		Signature   string `json:"signature"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if !decodePickerJSON(r, &in) {
		writeErr(w, 400, "invalid proof")
		return
	}
	c, ok := p.challenges[in.ChallengeID]
	delete(p.challenges, in.ChallengeID)
	if !ok || time.Now().Unix() >= c.Expires {
		writeErr(w, 401, "challenge expired")
		return
	}
	sig, e := base64.RawURLEncoding.DecodeString(in.Signature)
	hash := sha256.Sum256([]byte(c.Message))
	if e != nil || !ecdsa.VerifyASN1(c.Key, hash[:], sig) {
		writeErr(w, 401, "invalid signature")
		return
	}
	d, ok := p.config.Devices[c.Device.ID]
	if ok && d.Revoked {
		writeErr(w, 403, "device revoked")
		return
	}
	if !ok || !d.Approved && time.Now().Unix() >= d.Expires {
		for k, v := range p.config.Devices {
			if !v.Approved && !v.Revoked && v.Expires <= time.Now().Unix() {
				delete(p.config.Devices, k)
			}
		}
		if len(p.config.Devices) >= 128 {
			writeErr(w, 429, "device capacity")
			return
		}
		d = c.Device
		p.config.Devices[d.ID] = d
		if e = p.save(); e != nil {
			writeErr(w, 409, "state unavailable")
			return
		}
	}
	if !d.Approved {
		p.showDevicePrompt(d, s.listen)
		writeJSON(w, 202, map[string]string{"state": "approval_required", "device_id": d.ID})
		return
	}
	if p.config.Seen[c.JobID] && p.config.Owners[c.JobID] != d.ID {
		writeErr(w, 403, "job owner mismatch")
		return
	}
	now := time.Now().Unix()
	for k, g := range p.grants {
		if !p.validGrant(g) {
			delete(p.grants, k)
		}
	}
	if len(p.grants) >= 512 {
		writeErr(w, 429, "grant capacity")
		return
	}
	token, e := GenerateToken()
	if e != nil {
		writeErr(w, 409, "random source unavailable")
		return
	}
	p.grants[SHA256Bytes([]byte(token))] = powerGrant{DeviceID: d.ID, JobID: c.JobID, Expires: now + 7200, Epoch: p.pairEpoch}
	writeJSON(w, 200, map[string]any{"token": token, "expires_unix": now + 7200, "scope": "power:manual", "job_id": c.JobID})
}

// A single bounded native prompt prevents concurrent untrusted enrollment windows.
func (p *powerManager) showDevicePrompt(d powerDevice, listen string) {
	active := p.config.Devices[p.devicePrompt.ID]
	if p.devicePrompt.ID != "" && !active.Approved && !active.Revoked && time.Now().Unix() < p.devicePrompt.Expires {
		return
	}
	_, port, e := net.SplitHostPort(listen)
	if e != nil {
		return
	}
	token, e := GenerateToken()
	if e != nil {
		return
	}
	p.devicePrompt = powerDevicePrompt{ID: d.ID, Hash: SHA256Bytes([]byte(token)), Expires: d.Expires}
	if e = p.launch("http://127.0.0.1:" + port + "/api/v1/power/device-desktop#" + token); e != nil {
		p.devicePrompt = powerDevicePrompt{}
	}
}
func (s *HTTPServer) powerDeviceDesktop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !pickerLocal(r) || !browserWrite(r) {
		writeErr(w, 403, "local desktop required")
		return
	}
	p := s.power
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.devicePrompt
	if v.ID == "" || time.Now().Unix() >= v.Expires || !powerToken(r, v.Hash) {
		writeErr(w, 401, "device capability expired")
		return
	}
	var in struct {
		Action string `json:"action"`
		JobID  string `json:"job_id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if !decodePickerJSON(r, &in) {
		writeErr(w, 400, "invalid device action")
		return
	}
	d, ok := p.config.Devices[v.ID]
	if !ok || !p.config.PairingEnabled {
		writeErr(w, 409, "pairing no longer available")
		return
	}
	switch in.Action {
	case "heartbeat", "pause":
	case "approve", "reject":
		if d.Approved || d.Revoked {
			writeErr(w, 409, "device already decided")
			return
		}
		action := "approve_device"
		if in.Action == "reject" {
			action = "revoke_device"
		}
		if e := p.deviceAction(action, v.ID); e != nil {
			writeErr(w, 409, e.Error())
			return
		}
		d = p.config.Devices[v.ID]
	default:
		writeErr(w, 400, "invalid device action")
		return
	}
	state := "waiting"
	message := "請核對手機顯示的裝置指紋，再允許此裝置請求關機。"
	if d.Approved {
		state = "approved"
		message = "裝置已授權，之後不需重複配對。"
	} else if d.Revoked {
		state = "rejected"
		message = "已拒絕此裝置。"
	}
	writeJSON(w, 200, map[string]any{"theme": p.config.Theme, "device": d, "job": map[string]any{"job_id": d.ID, "state": state, "message": message}, "server_ms": time.Now().UnixMilli()})
}
