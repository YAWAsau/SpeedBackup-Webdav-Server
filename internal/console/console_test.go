package console

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
)

func TestFitAndRedirectedOutput(t *testing.T) {
	if got := Fit("中文abc", 5); got != "中文a" {
		t.Fatal(got)
	}
	if got := Fit("a\x1b\n\u202eb", 80); strings.ContainsAny(got, "\x1b\n\u202e") {
		t.Fatal(got)
	}
	f, e := os.CreateTemp(t.TempDir(), "output")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	draw, restore := Renderer(f, false)
	if e = draw([]string{"first", "second"}); e != nil {
		t.Fatal(e)
	}
	restore()
	b, e := os.ReadFile(f.Name())
	if e != nil || strings.Contains(string(b), "\x1b") || !strings.Contains(string(b), "second") {
		t.Fatal(string(b), e)
	}
}
func TestInputLimitsAndCancellation(t *testing.T) {
	got, e := ReadLine(context.Background(), bufio.NewReader(strings.NewReader(" password \r\n")))
	if e != nil || got != " password " {
		t.Fatal(got, e)
	}
	if _, e = ReadLine(context.Background(), bufio.NewReader(strings.NewReader(strings.Repeat("x", 8193)))); e == nil {
		t.Fatal("unbounded credential input")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = ReadLine(ctx, bufio.NewReader(strings.NewReader("")))
	if e == nil {
		t.Fatal("expected cancellation/EOF")
	}
}
