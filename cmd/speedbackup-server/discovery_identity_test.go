package main

import (
	"os"
	"speedbackup-server/internal/sbserver"
	"strings"
	"testing"
)

func TestResetTokenPreservesDiscoveryIdentity(t *testing.T) {
	root := t.TempDir()
	oldToken, created, err := sbserver.InitRoot(root)
	if err != nil || !created {
		t.Fatal("init", err)
	}
	store, err := sbserver.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	before.DiscoveryDisabled = true
	if err = store.SaveConfig(before); err != nil {
		t.Fatal(err)
	}
	capture, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = capture
	defer func() { os.Stdout = original; capture.Close() }()
	err = runResetToken([]string{"--root", root})
	os.Stdout = original
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(capture.Name())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	newToken := strings.TrimSpace(lines[len(lines)-1])
	after, err := store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.DiscoveryID) != 32 || after.DiscoveryID != before.DiscoveryID || !after.DiscoveryDisabled || newToken == oldToken || after.TokenSHA256 != sbserver.SHA256Bytes([]byte(newToken)) {
		t.Fatal("CLI rotation changed identity/settings or failed token persistence")
	}
}
