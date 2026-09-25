package sbserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func CleanRoot(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("--root is required")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return "", err
	}
	return abs, nil
}

// InitRoot creates server metadata and the authentication token only if the
// root has never been initialized. Existing installations keep their token.
func InitRoot(root string) (token string, created bool, err error) {
	return InitRootWithTokenFile(root, "")
}

// InitRootWithTokenFile holds the root lock through token delivery and config
// publication. A failed token-file write must not initialize an unusable root.
// Existing roots remain idempotent and never rewrite an operator's token file.
func InitRootWithTokenFile(root, tokenFile string) (token string, created bool, err error) {
	root, err = CleanRoot(root)
	if err != nil {
		return "", false, err
	}
	lock, err := AcquireRootLock(root)
	if err != nil {
		return "", false, err
	}
	defer lock.Close()
	store, err := NewStore(root)
	if err != nil {
		return "", false, err
	}
	if _, err = store.LoadConfig(); err == nil {
		return "", false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	token, err = GenerateToken()
	if err != nil {
		return "", false, err
	}
	// Persist the only plaintext copy before publishing its hash. If the process
	// is interrupted here, the operator still has the token and no hidden config.
	if err = writeFirstRunTokenFile(tokenFile, token); err != nil {
		return "", false, err
	}
	cfg := ServerConfig{Schema: "speedbackup.server.config.v1", TokenSHA256: SHA256Bytes([]byte(token)), CreatedUnix: nowUnix()}
	if err = store.SaveConfig(cfg); err != nil {
		if tokenFile != "" {
			if removeErr := os.Remove(tokenFile); removeErr != nil {
				return "", false, fmt.Errorf("save config: %w; remove uncommitted token file: %v", err, removeErr)
			}
		}
		return "", false, err
	}
	return token, true, nil
}

func writeFirstRunTokenFile(path, token string) error {
	if path == "" || token == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("token file already exists: %s", path)
	}
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err = io.WriteString(f, token+"\n"); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

// RunServer runs one SpeedBackup Server instance until ctx is cancelled.
// The root lock is held for the complete lifetime of the server.
func RunServer(ctx context.Context, root, listen string, out io.Writer) error {
	return RunServerWithReady(ctx, root, listen, out, nil)
}

// RunServerWithReady reports service readiness only after bind and auth setup.
func RunServerWithReady(ctx context.Context, root, listen string, out io.Writer, ready func() error) (result error) {
	root, err := CleanRoot(root)
	if err != nil {
		return err
	}
	lock, err := AcquireRootLock(root)
	if err != nil {
		return err
	}
	defer lock.Close()
	store, err := NewStore(root)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", listen)
	defer func() {
		if result != nil {
			store.diagnostics.append(map[string]any{"kind": "server_error", "time": time.Now().Format(time.RFC3339Nano), "version": ServerVersion, "message": result.Error()})
		}
		_ = store.diagnostics.flush()
	}()
	if err != nil {
		return err
	}
	defer ln.Close()
	cfg, firstToken, err := store.LoadOrInitConfig()
	if err != nil {
		return err
	}
	actual := ln.Addr().String()
	handler := NewHTTPHandler(store, actual, cfg.TokenSHA256, nil)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second, ErrorLog: log.New(diagnosticWriter{store.diagnostics}, "", 0)}

	if out != nil {
		fmt.Fprintln(out, "============================================================")
		fmt.Fprintln(out, " SpeedBackup Server", ServerVersion)
		fmt.Fprintln(out, "============================================================")
		fmt.Fprintln(out, "Root     :", root)
		fmt.Fprintln(out, "Listen   : http://"+actual)
		fmt.Fprintln(out, "WebAdmin : http://"+actual+"/web/admin")
		fmt.Fprintln(out, "Admin    : first visit via localhost to create an account; then sign in with username/password.")
		if firstToken != "" {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "FIRST-RUN TOKEN (shown once):")
			fmt.Fprintln(out, firstToken)
			fmt.Fprintln(out, "Legacy API token only; WebAdmin login does not require it.")
		}
		fmt.Fprintln(out, "============================================================")
	}

	if ready != nil {
		if err := ready(); err != nil {
			return err
		}
	}
	_ = store.AppendEvent(AuditEvent{Unix: time.Now().Unix(), Event: "server.start", Message: "server started", Details: map[string]any{"listen": actual, "root": root, "version": ServerVersion, "pid": os.Getpid()}})
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(ln) }()
	stopDiscovery := startDAVDiscovery(ctx, ln.Addr(), func(message string) {
		store.diagnostics.append(map[string]any{"kind": "lan_discovery", "time": time.Now().Format(time.RFC3339Nano), "message": message})
	})
	defer stopDiscovery()
	select {
	case <-ctx.Done():
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
		_ = store.AppendEvent(AuditEvent{Unix: time.Now().Unix(), Event: "server.stop", Message: "server stopped", Details: map[string]any{}})
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// RequireInitialized verifies that a service-mode root already has an auth
// configuration. Service mode must never generate a token into an invisible
// background log; install/init is responsible for provisioning it first.
func RequireInitialized(root string) error {
	root, err := CleanRoot(root)
	if err != nil {
		return err
	}
	store, err := NewStore(root)
	if err != nil {
		return err
	}
	if _, err = store.LoadConfig(); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("server root is not initialized; run 'speedbackup-server init --root %s' or 'service install' first", root)
		}
		return err
	}
	return nil
}
