//go:build windows

package desktop

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// Shell URL handling adds a root slash even when the browser's input had none.
// Exercise the Windows normalizer, not only a hand-written equivalent URL.
func TestLaunchAfterWindowsCanonicalization(t *testing.T) {
	canonicalize := syscall.NewLazyDLL("shlwapi.dll").NewProc("UrlCanonicalizeW")
	id := strings.Repeat("a", 64)
	for _, hostPath := range []string{"choose", "choose/"} {
		raw := Protocol + "://" + hostPath + "?port=8765&request=" + id
		input, err := syscall.UTF16PtrFromString(raw)
		if err != nil {
			t.Fatal(err)
		}
		output := make([]uint16, 4096)
		size := uint32(len(output))
		hr, _, _ := canonicalize.Call(uintptr(unsafe.Pointer(input)), uintptr(unsafe.Pointer(&output[0])), uintptr(unsafe.Pointer(&size)), 0)
		if err := hresult(uint32(hr)); err != nil {
			t.Fatal(err)
		}
		normalized := syscall.UTF16ToString(output)
		port, gotID, err := ParseLaunch(normalized)
		if err != nil || port != 8765 || gotID != id {
			t.Fatalf("Windows-normalized launch rejected: %q: port=%d err=%v", normalized, port, err)
		}
	}
}

func TestListenerOwner(t *testing.T) {
	data := make([]byte, 28)
	binary.LittleEndian.PutUint32(data, 1)
	binary.BigEndian.PutUint16(data[12:], 8765)
	binary.LittleEndian.PutUint32(data[24:], 1234)
	if !tcpOwnsPort(data, 8765, 1234) || tcpOwnsPort(data, 8765, 4321) || tcpOwnsPort(data, 80, 1234) || tcpOwnsPort(data[:27], 8765, 1234) {
		t.Fatal("listener identity/length check failed")
	}
	binary.LittleEndian.PutUint32(data[8:], 0x0200000a)
	if tcpOwnsPort(data, 8765, 1234) {
		t.Fatal("nonloopback endpoint accepted")
	}
	dual := make([]byte, 60)
	binary.LittleEndian.PutUint32(dual, 1)
	binary.BigEndian.PutUint16(dual[24:], 8765)
	binary.LittleEndian.PutUint32(dual[56:], 1234)
	if !tcp6OwnsPort(dual, 8765, 1234) || tcp6OwnsPort(dual, 8765, 5678) || tcp6OwnsPort(dual[:59], 8765, 1234) {
		t.Fatal("dual-stack listener identity/length check failed")
	}
}

// Creates/releases the real COM object without showing a modal desktop window.
func TestNativeDialogCOM(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 2)
	if err := hresult(uint32(r)); err != nil {
		t.Fatal(err)
	}
	defer ole32.NewProc("CoUninitialize").Call()
	dialog, err := openFolderDialog()
	if err != nil {
		t.Fatal(err)
	}
	defer dialog.call(2)
	if err = hresult(dialog.call(9, 0x20|0x40|0x800|0x02000000)); err != nil {
		t.Fatal(err)
	}
	if Available() {
		t.Fatal("test process must never control the installed service")
	}
}

// Opt-in manual smoke test: select the initial empty fixture directory, then
// repeat with SB_PICKER_INTERACTIVE=cancel and press Cancel. Never shares it.
func TestNativeDialogInteractive(t *testing.T) {
	mode := os.Getenv("SB_PICKER_INTERACTIVE")
	if mode != "select" && mode != "cancel" {
		t.Skip("interactive desktop test disabled")
	}
	want := filepath.Join(t.TempDir(), "資料夾 with spaces")
	if err := os.Mkdir(want, 0700); err != nil {
		t.Fatal(err)
	}
	got, cancelled, err := ChooseFolder(want)
	if err != nil {
		t.Fatal(err)
	}
	if mode == "cancel" {
		if !cancelled || got != "" {
			t.Fatalf("cancel returned %q, %v", got, cancelled)
		}
	} else if cancelled || !strings.EqualFold(filepath.Clean(got), want) {
		t.Fatalf("selection returned %q, cancelled=%v; want %q", got, cancelled, want)
	}
}
