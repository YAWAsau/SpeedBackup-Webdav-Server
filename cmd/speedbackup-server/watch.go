package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"speedbackup-server/internal/console"
	"speedbackup-server/internal/sbserver"
	"strings"
	"syscall"
	"time"
)

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	endpoint := fs.String("url", "http://127.0.0.1:8765", "running server origin")
	user := fs.String("username", "", "WebAdmin administrator name (prompt if omitted)")
	tokenFile := fs.String("token-file", "", "optional existing valid API token file")
	passwordStdin := fs.Bool("password-stdin", false, "read password from redirected stdin, requires --username")
	interval := fs.Duration("interval", time.Second, "refresh interval, 250ms..1m")
	plain := fs.Bool("plain", false, "append snapshots without cursor control")
	once := fs.Bool("once", false, "print one snapshot and exit")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("watch accepts flags only")
	}
	if *interval < 250*time.Millisecond || *interval > time.Minute {
		return fmt.Errorf("--interval must be between 250ms and 1m")
	}
	url, e := sbserver.WatchURL(*endpoint)
	if e != nil {
		return e
	}
	if *tokenFile != "" && (*user != "" || *passwordStdin) {
		return fmt.Errorf("use either --token-file or administrator login")
	}
	if *passwordStdin && *user == "" {
		return fmt.Errorf("--password-stdin requires --username")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	options := sbserver.WatchOptions{URL: url, Username: *user, Interval: *interval, Once: *once}
	if *tokenFile != "" {
		b, e := os.ReadFile(*tokenFile)
		if e != nil {
			return fmt.Errorf("cannot read token file: %w", e)
		}
		options.Token = strings.TrimSpace(string(b))
		clear(b)
		if options.Token == "" || len(options.Token) > 8192 || strings.ContainsAny(options.Token, "\r\n") {
			return fmt.Errorf("invalid token file")
		}
	} else {
		in := bufio.NewReader(os.Stdin)
		if options.Username == "" {
			fmt.Fprint(os.Stderr, "管理員帳號（管理網頁的帳號）： ")
			options.Username, e = console.ReadLine(ctx, in)
			if e != nil {
				if ctx.Err() != nil {
					return nil
				}
				return e
			}
		}
		options.Username = strings.TrimSpace(options.Username)
		if *passwordStdin {
			options.Password, e = console.ReadLine(ctx, in)
		} else {
			fmt.Fprint(os.Stderr, "管理員密碼（不顯示）： ")
			options.Password, e = console.Password(ctx, in)
			fmt.Fprintln(os.Stderr)
		}
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		if options.Username == "" || options.Password == "" {
			return fmt.Errorf("administrator name and password are required")
		}
	}
	draw, restore := console.Renderer(os.Stdout, *plain || *once)
	defer restore()
	return sbserver.Watch(ctx, options, draw)
}
