//go:build linux

package service

import (
	"strings"
	"testing"
)

func TestAutostartProtocol(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
	}{{`{"enabled":true}`, true}, {`{"enabled":false}`, false}} {
		got, err := decodeAutostartRequest([]byte(tc.raw))
		if err != nil || got != tc.want {
			t.Fatal(got, err)
		}
	}
	for _, raw := range []string{`{}`, `null`, `{"enabled":null}`, `{"enabled":"true"}`, `{"enabled":true,"unit":"other"}`, `{"enabled":true} {}`, strings.Repeat(" ", 129)} {
		if _, err := decodeAutostartRequest([]byte(raw)); err == nil {
			t.Fatal("accepted", raw)
		}
	}
}
