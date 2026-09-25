// Package console owns terminal changes only for the optional local viewer.
package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

func ReadLine(ctx context.Context, in *bufio.Reader) (string, error) {
	type result struct {
		s string
		e error
	}
	done := make(chan result, 1)
	go func() {
		var b strings.Builder
		for {
			c, e := in.ReadByte()
			if e != nil {
				if e == io.EOF && b.Len() > 0 {
					e = nil
				}
				done <- result{strings.TrimSuffix(b.String(), "\r"), e}
				return
			}
			if c == '\n' {
				done <- result{strings.TrimSuffix(b.String(), "\r"), nil}
				return
			}
			if b.Len() >= 8192 {
				done <- result{"", fmt.Errorf("input exceeds 8192 bytes")}
				return
			}
			b.WriteByte(c)
		}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-done:
		return r.s, r.e
	}
}
func Password(ctx context.Context, in *bufio.Reader) (string, error) {
	restore, e := hideInput(os.Stdin)
	if e != nil {
		return "", fmt.Errorf("cannot hide password input; use --password-stdin for redirected input")
	}
	defer restore()
	return ReadLine(ctx, in)
}

// Fit keeps CJK/wide glyphs within the visible viewport. Control sequences and
// bidi formatters from untrusted file names are never sent to the terminal.
func Fit(s string, width int) string {
	if width < 1 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			r = ' '
		}
		n := 1
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			n = 0
		} else if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe10 && r <= 0xfe6f) || (r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) || r >= 0x1f300) {
			n = 2
		}
		if used+n > width {
			break
		}
		b.WriteRune(r)
		used += n
	}
	return b.String()
}

func Renderer(out *os.File, plain bool) (func([]string) error, func()) {
	restore, interactive := enableOutput(out)
	if plain || !interactive {
		restore()
		return func(lines []string) error { _, e := fmt.Fprintln(out, strings.Join(lines, "\n")+"\n"); return e }, func() {}
	}
	started := false
	return func(lines []string) error {
			w, h := dimensions(out)
			if w < 10 {
				w = 80
			}
			if h < 5 {
				h = 24
			}
			var b strings.Builder
			if !started {
				b.WriteString("\x1b[?1049h\x1b[?25l")
				started = true
			}
			b.WriteString("\x1b[H")
			limit := h - 1
			if len(lines) > limit {
				lines = append(append([]string{}, lines[:limit-1]...), "… 更多項目請放大終端或使用 --plain")
			}
			for _, line := range lines {
				b.WriteString("\x1b[2K")
				b.WriteString(Fit(line, w-1))
				b.WriteString("\r\n")
			}
			b.WriteString("\x1b[J")
			_, e := io.WriteString(out, b.String())
			return e
		}, func() {
			if started {
				fmt.Fprint(out, "\x1b[?25h\x1b[?1049l")
			}
			restore()
		}
}
