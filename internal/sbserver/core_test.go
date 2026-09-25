package sbserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLogicalPath(t *testing.T) {
	good := []string{"app/com.example/base.tar.zst", "media/DCIM/a.jpg", "wifi/config.json"}
	for _, p := range good {
		if err := ValidateLogicalPath(p); err != nil {
			t.Fatalf("valid path %q rejected: %v", p, err)
		}
	}
	bad := []string{"", "/abs", "../escape", "a/../b", "a//b", "a\\b", "./a", "a/./b"}
	for _, p := range bad {
		if err := ValidateLogicalPath(p); err == nil {
			t.Fatalf("unsafe path %q accepted", p)
		}
	}
}

func TestAppDetailsAuditSeedlessTaint(t *testing.T) {
	got := AuditAppDetails(AppDetailsAuditRequest{
		SeedOK:      false,
		StageApps:   []string{"com.a", "com.b"},
		PayloadApps: []string{"com.a", "com.b", "wifi", "com.unknown"},
	})
	if !got.Allowed || !got.SeedlessRepair || !got.SeedlessTainted {
		t.Fatalf("unexpected audit: %+v", got)
	}
	if strings.Join(got.IgnoredRemotePayloadApps, ",") != "com.unknown,wifi" {
		t.Fatalf("ignored list=%v", got.IgnoredRemotePayloadApps)
	}
}

func TestAppDetailsAuditMissingStagePayloadBlocks(t *testing.T) {
	got := AuditAppDetails(AppDetailsAuditRequest{SeedOK: false, StageApps: []string{"com.a", "com.b"}, PayloadApps: []string{"com.a"}})
	if got.Allowed {
		t.Fatalf("audit should block: %+v", got)
	}
	if strings.Join(got.MissingStagePayloadApps, ",") != "com.b" {
		t.Fatalf("missing=%v", got.MissingStagePayloadApps)
	}
}

func TestAppendOnlySessionMetadata(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	m := SessionMeta{Schema: SessionSchema, ID: "sess1", DeviceID: "dev1", ProfileID: "default", State: "open", CreatedUnix: 1, UpdatedUnix: 1, Uploads: map[string]UploadedObject{}}
	if err := s.SaveSession(m); err != nil {
		t.Fatal(err)
	}
	m.State = "verified"
	m.UpdatedUnix = 2
	if err := s.SaveSession(m); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadSession("sess1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "verified" || got.UpdatedUnix != 2 {
		t.Fatalf("latest meta not loaded: %+v", got)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".speedbackup-server", "sessions", "sess1", "meta"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 metadata versions, got %d", len(entries))
	}
}

func TestNormalizeEntriesRejectDuplicatePath(t *testing.T) {
	hash := strings.Repeat("a", 64)
	_, err := NormalizeEntries([]ManifestEntry{{Path: "a", Size: 1, SHA256: hash}, {Path: "a", Size: 1, SHA256: hash}})
	if err == nil {
		t.Fatal("duplicate path accepted")
	}
}

func TestRootLockIsExclusive(t *testing.T) {
	root := t.TempDir()
	a, err := AcquireRootLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if b, err := AcquireRootLock(root); err == nil {
		_ = b.Close()
		t.Fatal("second root lock unexpectedly succeeded")
	}
}

func TestCleanupFailsClosedOnCorruptManifest(t *testing.T) {
	root := t.TempDir()
	s, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	p := s.generationPath("dev", "default", 1)
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CleanupObjects(true); err == nil {
		t.Fatal("cleanup accepted corrupt manifest")
	}
}

func TestInitRootIsIdempotentAndDoesNotRotateToken(t *testing.T) {
	root := t.TempDir()
	tok, created, err := InitRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if !created || tok == "" {
		t.Fatalf("first init created=%v token=%q", created, tok)
	}
	s, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	tok2, created2, err := InitRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if created2 || tok2 != "" {
		t.Fatalf("second init created=%v token=%q", created2, tok2)
	}
	after, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if before.TokenSHA256 != after.TokenSHA256 {
		t.Fatal("idempotent init rotated token")
	}
}

func TestRequireInitializedRejectsFreshRootAndAcceptsInitialized(t *testing.T) {
	root := t.TempDir()
	if err := RequireInitialized(root); err == nil {
		t.Fatal("fresh root accepted")
	}
	if _, _, err := InitRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := RequireInitialized(root); err != nil {
		t.Fatalf("initialized root rejected: %v", err)
	}
}
