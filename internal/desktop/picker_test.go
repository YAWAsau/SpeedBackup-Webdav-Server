package desktop

import (
	"strings"
	"testing"
)

func TestLaunchValidation(t *testing.T) {
	good := "speedbackup-picker://choose?port=8765&request=" + strings.Repeat("a", 64)
	port, id, err := ParseLaunch(good)
	if err != nil || port != 8765 || len(id) != 64 {
		t.Fatal(port, id, err)
	}
	canonical := strings.Replace(good, "choose?", "choose/?", 1)
	if port, gotID, err := ParseLaunch(canonical); err != nil || port != 8765 || gotID != id {
		t.Fatal("root slash rejected", port, err)
	}
	for _, path := range []string{"//", "/path", "/%2F", "%2F", "/..", "/%2e", "/\\evil"} {
		if _, _, err := ParseLaunch(strings.Replace(good, "choose?", "choose"+path+"?", 1)); err == nil {
			t.Errorf("accepted non-root path %q", path)
		}
	}
	for _, raw := range []string{strings.Replace(good, "8765", "0", 1), strings.Replace(good, "8765", "65536", 1), good + "&port=80", good + "&callback=http://evil", good + "#ignored", strings.Replace(good, "choose", "user@choose", 1), strings.Replace(good, "choose?", "choose/path?", 1), strings.Replace(good, "picker://", "picker:http://", 1), good[:len(good)-1], "https://example.com"} {
		if _, _, e := ParseLaunch(raw); e == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
