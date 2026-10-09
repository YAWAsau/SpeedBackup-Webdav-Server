package sbserver

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPowerDevicePairing(t *testing.T) {
	p := powerFixture(t)
	p.config.Enabled = false
	s := &HTTPServer{power: p, listen: "127.0.0.1:8765"}
	launches := []string{}
	p.launch = func(u string) error { launches = append(launches, u); return nil }
	call := func(h http.HandlerFunc, path string, body any, token string, code int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "http://127.0.0.1:8765"+path, bytes.NewReader(b))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-SB-Admin", "1")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h(w, r)
		if w.Code != code {
			t.Fatalf("%s: got %d want %d: %s", path, w.Code, code, w.Body.String())
		}
		v := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return v
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	id := SHA256Bytes(der)
	job := "device-job-000000001"
	challengeBody := map[string]string{"public_key": base64.RawURLEncoding.EncodeToString(der), "device_name": "test phone", "job_id": job, "scope": "power:manual"}
	call(s.powerChallenge, "/api/v1/power/challenge", challengeBody, "", 403)
	if e := p.deviceAction("enable_pairing", ""); e != nil {
		t.Fatal(e)
	}
	proof := func() map[string]string {
		t.Helper()
		c := call(s.powerChallenge, "/api/v1/power/challenge", challengeBody, "", 200)
		hash := sha256.Sum256([]byte(c["message"].(string)))
		sig, _ := ecdsa.SignASN1(rand.Reader, key, hash[:])
		return map[string]string{"challenge_id": c["challenge_id"].(string), "signature": base64.RawURLEncoding.EncodeToString(sig)}
	}
	bad := proof()
	bad["signature"] = "AA"
	call(s.powerPair, "/api/v1/power/pair", bad, "", 401)
	call(s.powerPair, "/api/v1/power/pair", bad, "", 401)
	request := proof()
	v := call(s.powerPair, "/api/v1/power/pair", request, "", 202)
	if v["device_id"] != id || len(launches) != 1 || !strings.Contains(launches[0], "/device-desktop#") {
		t.Fatal("native approval missing")
	}
	call(s.powerPair, "/api/v1/power/pair", request, "", 401)
	call(s.powerPair, "/api/v1/power/pair", proof(), "", 202)
	if len(launches) != 1 {
		t.Fatal("duplicate prompt")
	}
	desktop := strings.Split(launches[0], "#")[1]
	call(s.powerDeviceDesktop, "/api/v1/power/device-desktop", map[string]string{"action": "approve"}, "wrong", 401)
	call(s.powerDeviceDesktop, "/api/v1/power/device-desktop", map[string]string{"action": "approve"}, desktop, 200)
	v = call(s.powerPair, "/api/v1/power/pair", proof(), "", 200)
	token := v["token"].(string)
	if v["scope"] != "power:manual" || v["job_id"] != job {
		t.Fatal(v)
	}
	body := map[string]any{"job_id": job, "operation": "restore", "outcome": "failed", "user_confirmed": true, "preview": true}
	body["job_id"] = "different-job-000001"
	call(s.powerComplete, "/api/v1/power/manual", body, token, 403)
	body["job_id"] = job
	call(s.powerComplete, "/api/v1/power/complete", map[string]string{"job_id": job, "result": "success"}, token, 403)
	body["preview"] = false
	call(s.powerComplete, "/api/v1/power/manual", body, token, 409)
	body["preview"] = true
	call(s.powerComplete, "/api/v1/power/manual", body, token, 202)
	call(s.powerComplete, "/api/v1/power/manual", body, token, 200)
	if p.config.Owners[job] != id {
		t.Fatal("owner missing")
	}
	p.deviceAction("revoke_device", id)
	if p.job.State != "cancelled" {
		t.Fatal("revoke did not cancel")
	}
	call(s.powerComplete, "/api/v1/power/manual", body, token, 401)
	p.deviceAction("approve_device", id)
	call(s.powerComplete, "/api/v1/power/manual", body, token, 401)
	g := powerGrant{DeviceID: id, JobID: job, Expires: time.Now().Unix() - 1, Epoch: p.pairEpoch}
	if p.validGrant(g) {
		t.Fatal("expired grant accepted")
	}
	restarted := newPowerManager(p.store)
	defer restarted.close()
	if !restarted.config.Devices[id].Approved || !restarted.config.Seen[job] || restarted.config.Owners[job] != id || len(restarted.grants) != 0 {
		t.Fatal("restart contract")
	}
	// Deleting a paired phone must require fresh approval, even with the same key.
	v = call(s.powerPair, "/api/v1/power/pair", proof(), "", 200)
	oldToken := v["token"].(string)
	staleProof := proof()
	if e := p.deviceAction("delete_device", id); e != nil {
		t.Fatal(e)
	}
	call(s.powerPair, "/api/v1/power/pair", staleProof, "", 401)
	call(s.powerComplete, "/api/v1/power/manual", body, oldToken, 401)
	call(s.powerDeviceDesktop, "/api/v1/power/device-desktop", map[string]string{"action": "approve"}, desktop, 401)
	challengeBody["job_id"] = "fresh-device-job-000002"
	call(s.powerPair, "/api/v1/power/pair", proof(), "", 202)
	if p.config.Devices[id].Approved {
		t.Fatal("deleted device automatically approved")
	}
	challengeBody["job_id"] = job
	// Another approved key cannot adopt the durable job ID.
	key2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der2, _ := x509.MarshalPKIXPublicKey(&key2.PublicKey)
	challengeBody["public_key"] = base64.RawURLEncoding.EncodeToString(der2)
	call(s.powerChallenge, "/api/v1/power/challenge", challengeBody, "", 403)
}
