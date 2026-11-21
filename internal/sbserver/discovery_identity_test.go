package sbserver

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestDiscoveryIdentityMigrationAndRestart(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.diagnostics.flush()
	old := ServerConfig{Schema: "speedbackup.server.config.v1", TokenSHA256: "existing-auth-hash", CreatedUnix: 123, DiscoveryDisabled: true}
	if err = writeImmutableJSON(filepath.Join(s.ConfigDir(), "00000000000000000001.json"), old); err != nil {
		t.Fatal(err)
	}
	c, token, err := s.LoadOrInitConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !validDiscoveryID(c.DiscoveryID) || token != "" || c.TokenSHA256 != old.TokenSHA256 || c.CreatedUnix != 123 || !c.DiscoveryDisabled {
		t.Fatal("legacy migration changed authentication/settings")
	}
	version, _ := latestVersionFile(s.ConfigDir())
	for i := 0; i < 3; i++ {
		restarted, e := NewStore(root)
		if e != nil {
			t.Fatal(e)
		}
		again, tok, e := restarted.LoadOrInitConfig()
		restarted.diagnostics.flush()
		if e != nil || tok != "" || again.DiscoveryID != c.DiscoveryID {
			t.Fatal("identity changed across restart", e)
		}
	}
	after, _ := latestVersionFile(s.ConfigDir())
	if after != version {
		t.Fatal("unchanged startup rewrote config")
	}
	// Credential rotation can reconstruct config and omit the identity.
	if err = s.SaveConfig(ServerConfig{Schema: old.Schema, TokenSHA256: "rotated", CreatedUnix: 456}); err != nil {
		t.Fatal(err)
	}
	rotated, _ := s.LoadConfig()
	if rotated.DiscoveryID != c.DiscoveryID || rotated.TokenSHA256 != "rotated" {
		t.Fatal("rotation lost identity")
	}
	other, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer other.diagnostics.flush()
	separate, _, e := other.LoadOrInitConfig()
	if e != nil || !validDiscoveryID(separate.DiscoveryID) || separate.DiscoveryID == c.DiscoveryID {
		t.Fatal("different installations share identity", e)
	}
}
func TestDiscoveryIdentityInAllInterfaceRecords(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	first, _, err := discoveryServices(&net.TCPAddr{IP: net.IPv6zero, Port: 18767}, id)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := discoveryServices(&net.TCPAddr{IP: net.IPv6zero, Port: 18768}, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Skip("no multicast interfaces")
	}
	byName := map[string]string{}
	for _, s := range first {
		byName[s.Name] = s.Host
		if s.Text["id"] != id || s.Text["product"] != "SpeedBackup" || len(s.Text) != 4 {
			t.Fatal("invalid public TXT identity")
		}
	}
	for _, s := range second {
		if s.Text["id"] != id || byName[s.Name] != s.Host || s.Port != 18768 {
			t.Fatal("identity depends on TCP port")
		}
	}
}

func TestDiscoveryIdentityHTTPTokenRotation(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.diagnostics.flush()
	cfg, oldToken, err := store.LoadOrInitConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DiscoveryDisabled = true
	if err = store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	h := NewHTTPHandler(store, "127.0.0.1:0", cfg.TokenSHA256, nil)
	defer h.(*managedHTTPHandler).stop()
	ts := httptest.NewServer(h)
	defer ts.Close()
	resp := doReq(t, ts.Client(), "POST", ts.URL+"/api/v1/admin/token/rotate", oldToken, nil, nil)
	if resp.StatusCode != 200 {
		resp.Body.Close()
		t.Fatalf("rotation status %d", resp.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if body.Token == "" || body.Token == oldToken || after.DiscoveryID != cfg.DiscoveryID || !after.DiscoveryDisabled || after.TokenSHA256 != SHA256Bytes([]byte(body.Token)) {
		t.Fatal("HTTP rotation changed identity/settings or failed to persist authentication")
	}
	for _, tc := range []struct {
		token  string
		status int
	}{{oldToken, 401}, {body.Token, 200}} {
		resp = doReq(t, ts.Client(), "GET", ts.URL+"/api/v1/status", tc.token, nil, nil)
		resp.Body.Close()
		if resp.StatusCode != tc.status {
			t.Fatalf("auth transition status %d want %d", resp.StatusCode, tc.status)
		}
	}
}
