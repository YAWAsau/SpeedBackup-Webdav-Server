package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"speedbackup-server/internal/sbserver"
	winservice "speedbackup-server/internal/service"
)

func usage() {
	fmt.Fprintf(os.Stderr, `SpeedBackup Server %s

Usage:
  speedbackup-server serve --root <path> [--listen 0.0.0.0:8765]
  speedbackup-server logs --root <path> [--follow] [--lines 20]
  speedbackup-server watch [--url http://127.0.0.1:8765] [--username <admin>] [--interval 1s] [--plain] [--once]
  speedbackup-server debug-export --root <path> [--output <new.zip>]
  speedbackup-server init --root <path> [--token-file <path>]
  speedbackup-server reset-token --root <path>
  speedbackup-server admin-reset --root <path> --username <name> --password-stdin
  speedbackup-server service install --root <path> [--listen 0.0.0.0:8765] [--token-file <path>]
  speedbackup-server service start|stop|status|uninstall
  speedbackup-server service autostart --enabled=true|false
  speedbackup-server service run --root <path> [--listen 0.0.0.0:8765]
  speedbackup-server version
`, sbserver.ServerVersion)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe(os.Args[2:])
	case "logs":
		err = runLogs(os.Args[2:])
	case "watch":
		err = runWatch(os.Args[2:])
	case "debug-export":
		err = runDebugExport(os.Args[2:])
	case "init":
		err = runInit(os.Args[2:])
	case "reset-token":
		err = runResetToken(os.Args[2:])
	case "admin-reset":
		err = runAdminReset(os.Args[2:])
	case "service":
		err = runService(os.Args[2:])
	case "version", "--version", "-version":
		fmt.Printf("SpeedBackup Server %s protocol %d.%d\n", sbserver.ServerVersion, sbserver.ProtocolMajor, sbserver.ProtocolMinor)
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func parseRootListen(name string, args []string) (root, listen string, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	rootArg := fs.String("root", "", "backup root")
	listenArg := fs.String("listen", winservice.DefaultListen, "HTTP listen address")
	if err = fs.Parse(args); err != nil {
		return "", "", err
	}
	root, err = sbserver.CleanRoot(*rootArg)
	if err != nil {
		return "", "", err
	}
	return root, *listenArg, nil
}

func runServe(args []string) error {
	root, listen, err := parseRootListen("serve", args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return sbserver.RunServerWithReady(ctx, root, listen, os.Stdout, winservice.NotifyReady)
}

func runLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	rootArg := fs.String("root", "", "existing server data root (not the shared backup directory)")
	follow := fs.Bool("follow", false, "keep reading completed transfers and server events; Ctrl+C exits only this viewer")
	lines := fs.Int("lines", 20, "number of recent events, 0-1000")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *lines < 0 || *lines > 1000 {
		return fmt.Errorf("use --lines 0..1000 without positional arguments")
	}
	if *rootArg == "" {
		return fmt.Errorf("--root is required")
	}
	root, err := filepath.Abs(*rootArg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return sbserver.StreamAuditLog(ctx, root, os.Stdout, *lines, *follow)
}

func runDebugExport(args []string) error {
	fs := flag.NewFlagSet("debug-export", flag.ContinueOnError)
	root := fs.String("root", "", "existing server data root")
	output := fs.String("output", "speedbackup_server_debug_"+time.Now().Format("20060102-150405")+".zip", "new ZIP path; existing files are never replaced")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *root == "" || fs.NArg() != 0 {
		return fmt.Errorf("--root is required without positional arguments")
	}
	f, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = sbserver.WriteDebugBundle(*root, f, nil)
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		_ = os.Remove(*output)
		return err
	}
	fmt.Println(*output)
	return nil
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	rootArg := fs.String("root", "", "backup root")
	tokenFileArg := fs.String("token-file", "", "write first-run token to this file on fresh initialization")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := sbserver.CleanRoot(*rootArg)
	if err != nil {
		return err
	}
	token, created, err := sbserver.InitRootWithTokenFile(root, *tokenFileArg)
	if err != nil {
		return err
	}
	if !created {
		fmt.Println("ALREADY_INITIALIZED")
		return nil
	}
	fmt.Println("FIRST-RUN TOKEN (shown once):")
	fmt.Println(token)
	if *tokenFileArg != "" {
		fmt.Println("FIRST-RUN TOKEN FILE:", *tokenFileArg)
	}
	return nil
}

func runResetToken(args []string) error {
	fs := flag.NewFlagSet("reset-token", flag.ContinueOnError)
	rootArg := fs.String("root", "", "backup root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := sbserver.CleanRoot(*rootArg)
	if err != nil {
		return err
	}
	lock, err := sbserver.AcquireRootLock(root)
	if err != nil {
		return err
	}
	defer lock.Close()
	store, err := sbserver.NewStore(root)
	if err != nil {
		return err
	}
	token, err := sbserver.GenerateToken()
	if err != nil {
		return err
	}
	cfg := sbserver.ServerConfig{Schema: "speedbackup.server.config.v1", TokenSHA256: sbserver.SHA256Bytes([]byte(token)), CreatedUnix: time.Now().Unix()}
	if err = store.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Println("NEW TOKEN (shown once):")
	fmt.Println(token)
	return nil
}

func runAdminReset(args []string) error {
	fs := flag.NewFlagSet("admin-reset", flag.ContinueOnError)
	rootArg := fs.String("root", "", "server data root")
	user := fs.String("username", "admin", "administrator username")
	stdin := fs.Bool("password-stdin", false, "read new password from stdin; never pass password as an argument")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if !*stdin {
		return fmt.Errorf("use --password-stdin; stop the server before resetting the administrator")
	}
	root, e := sbserver.CleanRoot(*rootArg)
	if e != nil {
		return e
	}
	if e = sbserver.RequireInitialized(root); e != nil {
		return e
	}
	lock, e := sbserver.AcquireRootLock(root)
	if e != nil {
		return e
	}
	defer lock.Close()
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if e := scanner.Err(); e != nil {
			return fmt.Errorf("read password: %w", e)
		}
		return fmt.Errorf("password required on stdin")
	}
	store, e := sbserver.NewStore(root)
	if e != nil {
		return e
	}
	if e = store.SetAdministrator(*user, strings.TrimSuffix(scanner.Text(), "\r")); e != nil {
		return e
	}
	fmt.Println("Administrator saved. Start the server and sign in with the new password. Backup accounts and files are unchanged.")
	return nil
}

func runService(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("service command required: install|start|stop|status|uninstall|run")
	}
	switch args[0] {
	case "autostart-agent":
		if len(args) != 1 {
			return fmt.Errorf("autostart-agent takes no arguments")
		}
		return winservice.RunAutostartAgent()
	case "autostart":
		fs := flag.NewFlagSet("service autostart", flag.ContinueOnError)
		value := fs.String("enabled", "", "enable or disable startup at boot")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		enabled, err := strconv.ParseBool(*value)
		if err != nil || fs.NArg() != 0 {
			return fmt.Errorf("use service autostart --enabled=true|false")
		}
		return winservice.SetAutostart(enabled)
	case "install":
		fs := flag.NewFlagSet("service install", flag.ContinueOnError)
		rootArg := fs.String("root", "", "backup root")
		listenArg := fs.String("listen", winservice.DefaultListen, "HTTP listen address")
		tokenFileArg := fs.String("token-file", "", "write first-run token to this file on fresh initialization")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		root, err := sbserver.CleanRoot(*rootArg)
		if err != nil {
			return err
		}
		listen := *listenArg
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if _, err = winservice.ValidateServiceInstallArgs(exe, root, listen); err != nil {
			return err
		}
		token, created, err := sbserver.InitRootWithTokenFile(root, *tokenFileArg)
		if err != nil {
			return err
		}
		if created {
			fmt.Println("FIRST-RUN TOKEN (shown once):")
			fmt.Println(token)
			if *tokenFileArg != "" {
				fmt.Println("FIRST-RUN TOKEN FILE:", *tokenFileArg)
			}
		}
		if err = winservice.Install(exe, root, listen); err != nil {
			return err
		}
		fmt.Println("Windows service installed:", winservice.WindowsServiceName)
		return nil
	case "start":
		if len(args) != 1 {
			return fmt.Errorf("service start takes no additional arguments")
		}
		return winservice.Start()
	case "stop":
		if len(args) != 1 {
			return fmt.Errorf("service stop takes no additional arguments")
		}
		return winservice.Stop()
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("service status takes no additional arguments")
		}
		s, err := winservice.Status()
		if s != "" {
			fmt.Print(s)
		}
		return err
	case "uninstall":
		if len(args) != 1 {
			return fmt.Errorf("service uninstall takes no additional arguments")
		}
		return winservice.Uninstall()
	case "run":
		root, listen, err := parseRootListen("service run", args[1:])
		if err != nil {
			return err
		}
		if err = sbserver.RequireInitialized(root); err != nil {
			return err
		}
		return winservice.Run(root, listen, func(ctx context.Context, out io.Writer, ready func() error) error {
			return sbserver.RunServerWithReady(ctx, root, listen, out, ready)
		})
	default:
		return fmt.Errorf("unknown service command %q", args[0])
	}
}
