package sbserver

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRootWithTokenFileWritesTokenOnlyOnFirstInit(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "data")
	tokenFile := filepath.Join(dir, "FIRST_RUN_TOKEN.txt")

	token, created, err := InitRootWithTokenFile(root, tokenFile)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	if !created {
		t.Fatal("first init was not marked created")
	}
	if !strings.HasPrefix(token, "sb1_") {
		t.Fatalf("unexpected token prefix: %q", token)
	}
	b, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("token file missing: %v", err)
	}
	if strings.TrimSpace(string(b)) != token {
		t.Fatalf("token file mismatch got %q want %q", strings.TrimSpace(string(b)), token)
	}

	if err := os.WriteFile(tokenFile, []byte("operator-copy\n"), 0600); err != nil {
		t.Fatalf("overwrite fixture token file: %v", err)
	}
	token2, created2, err := InitRootWithTokenFile(root, tokenFile)
	if err != nil {
		t.Fatalf("second init failed: %v", err)
	}
	if created2 {
		t.Fatal("second init should not create a new token")
	}
	if token2 != "" {
		t.Fatalf("second init returned token %q", token2)
	}
	b, err = os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("token file lost after second init: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "operator-copy" {
		t.Fatalf("existing token file was modified: %q", got)
	}
}

func TestInitRootWithTokenFileDoesNotOverwriteExistingFileOnFreshInit(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "data")
	tokenFile := filepath.Join(dir, "FIRST_RUN_TOKEN.txt")
	if err := os.WriteFile(tokenFile, []byte("existing\n"), 0600); err != nil {
		t.Fatalf("write existing token file: %v", err)
	}

	_, _, err := InitRootWithTokenFile(root, tokenFile)
	if err == nil || !strings.Contains(err.Error(), "token file already exists") {
		t.Fatalf("want token file already exists error, got %v", err)
	}
	b, _ := os.ReadFile(tokenFile)
	if got := strings.TrimSpace(string(b)); got != "existing" {
		t.Fatalf("existing token file was overwritten: %q", got)
	}
	if err := RequireInitialized(root); err == nil {
		t.Fatal("failed token delivery must not initialize the root")
	}
	if _, created, err := InitRootWithTokenFile(root, filepath.Join(dir, "retry.txt")); err != nil || !created {
		t.Fatalf("retry must create a usable root: created=%v err=%v", created, err)
	}
}

func TestRunServerListenFailureDoesNotInitializeRoot(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	root := t.TempDir()
	if err := RunServer(context.Background(), root, ln.Addr().String(), nil); err == nil {
		t.Fatal("expected bind failure")
	}
	if err := RequireInitialized(root); err == nil {
		t.Fatal("bind failure must not create an undisclosed token")
	}
}

func TestReadyRunsOnlyAfterRootAndListenerAreReady(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := false
	err := RunServerWithReady(ctx, root, "127.0.0.1:0", nil, func() error {
		called = true
		if err := RequireInitialized(root); err != nil {
			t.Error(err)
		}
		cancel()
		return nil
	})
	if err != nil || !called {
		t.Fatalf("readiness callback: called=%v err=%v", called, err)
	}
}
