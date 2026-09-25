//go:build windows

package desktop

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"speedbackup-server/internal/service"
	"syscall"
	"unsafe"
)

var ole32 = syscall.NewLazyDLL("ole32.dll")
var shell32 = syscall.NewLazyDLL("shell32.dll")
var advapi32 = syscall.NewLazyDLL("advapi32.dll")

// GUID layout used by COM; pointers remain Go-owned only for the call duration.
type guid struct {
	A    uint32
	B, C uint16
	D    [8]byte
}

var dialogClass = guid{0xdc1c5a9c, 0xe88a, 0x4dde, [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
var dialogIID = guid{0xd57c7288, 0xd4ad, 0x4768, [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
var itemIID = guid{0x43826d1e, 0xe718, 0x42ee, [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}

type comObject struct{ table *[32]uintptr }

func (o *comObject) call(method int, args ...uintptr) uint32 {
	a := append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	r, _, _ := syscall.SyscallN(o.table[method], a...)
	runtime.KeepAlive(o)
	return uint32(r)
}
func hresult(r uint32) error {
	if int32(r) < 0 {
		return fmt.Errorf("Windows folder dialog: 0x%08x", r)
	}
	return nil
}

func openFolderDialog() (*comObject, error) {
	var dialog *comObject
	r, _, _ := ole32.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&dialogClass)), 0, 1, uintptr(unsafe.Pointer(&dialogIID)), uintptr(unsafe.Pointer(&dialog)))
	if err := hresult(uint32(r)); err != nil {
		return nil, err
	}
	return dialog, nil
}

func ChooseFolder(initial string) (string, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 2)
	if err := hresult(uint32(r)); err != nil {
		return "", false, err
	}
	defer ole32.NewProc("CoUninitialize").Call()
	dialog, err := openFolderDialog()
	if err != nil {
		return "", false, err
	}
	defer dialog.call(2)
	if err = hresult(dialog.call(9, 0x20|0x40|0x800|0x02000000)); err != nil {
		return "", false, err
	}
	title, _ := syscall.UTF16PtrFromString("選擇 SpeedBackup 分享資料夾")
	dialog.call(17, uintptr(unsafe.Pointer(title)))
	runtime.KeepAlive(title)
	if initial != "" {
		path, e := syscall.UTF16PtrFromString(initial)
		if e == nil {
			var item *comObject
			hr, _, _ := shell32.NewProc("SHCreateItemFromParsingName").Call(uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&itemIID)), uintptr(unsafe.Pointer(&item)))
			if hresult(uint32(hr)) == nil {
				dialog.call(12, uintptr(unsafe.Pointer(item)))
				item.call(2)
			}
		}
	}
	hr := dialog.call(3, 0)
	if hr == 0x800704c7 {
		return "", true, nil
	}
	if err = hresult(hr); err != nil {
		return "", false, err
	}
	var item *comObject
	if err = hresult(dialog.call(20, uintptr(unsafe.Pointer(&item)))); err != nil {
		return "", false, err
	}
	defer item.call(2)
	var name *uint16
	if err = hresult(item.call(5, 0x80058000, uintptr(unsafe.Pointer(&name)))); err != nil {
		return "", false, err
	}
	defer ole32.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(name)))
	// Windows filesystem paths are limited to 32767 UTF-16 code units.
	units := []uint16{}
	for i := uintptr(0); i < 32768; i++ {
		c := *(*uint16)(unsafe.Add(unsafe.Pointer(name), i*2))
		if c == 0 {
			return syscall.UTF16ToString(units), false, nil
		}
		units = append(units, c)
	}
	return "", false, fmt.Errorf("folder path is too long")
}

func ShowError(err error) {
	text, _ := syscall.UTF16PtrFromString(err.Error())
	title, _ := syscall.UTF16PtrFromString("SpeedBackup 資料夾選擇")
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}

func servicePID() uint32 {
	scm, _, _ := advapi32.NewProc("OpenSCManagerW").Call(0, 0, 1)
	if scm == 0 {
		return 0
	}
	defer advapi32.NewProc("CloseServiceHandle").Call(scm)
	name, _ := syscall.UTF16PtrFromString(service.WindowsServiceName)
	svc, _, _ := advapi32.NewProc("OpenServiceW").Call(scm, uintptr(unsafe.Pointer(name)), 4)
	if svc == 0 {
		return 0
	}
	defer advapi32.NewProc("CloseServiceHandle").Call(svc)
	var status [9]uint32
	var needed uint32
	ok, _, _ := advapi32.NewProc("QueryServiceStatusEx").Call(svc, 0, uintptr(unsafe.Pointer(&status)), unsafe.Sizeof(status), uintptr(unsafe.Pointer(&needed)))
	if ok != 0 && status[1] == 4 {
		return status[7]
	}
	return 0
}

func Available() bool {
	if servicePID() != uint32(os.Getpid()) {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	helper := filepath.Join(filepath.Dir(exe), "speedbackup-picker.exe")
	if st, e := os.Stat(helper); e != nil || st.IsDir() {
		return false
	}
	// Check the system-wide installed protocol, not a potentially unrelated URL handler.
	key, _ := syscall.UTF16PtrFromString(`Software\Classes\` + Protocol + `\shell\open\command`)
	var handle syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, key, 0, syscall.KEY_READ|syscall.KEY_WOW64_64KEY, &handle) != nil {
		return false
	}
	defer syscall.RegCloseKey(handle)
	var kind, size uint32
	buffer := make([]uint16, 32768)
	size = uint32(len(buffer) * 2)
	if syscall.RegQueryValueEx(handle, nil, nil, &kind, (*byte)(unsafe.Pointer(&buffer[0])), &size) != nil || kind != syscall.REG_SZ {
		return false
	}
	return syscall.UTF16ToString(buffer) == `"`+helper+`" "%1"`
}

func tcpOwnsPort(table []byte, port int, pid uint32) bool {
	if len(table) < 4 {
		return false
	}
	count := int(binary.LittleEndian.Uint32(table))
	if count > (len(table)-4)/24 {
		return false
	}
	for i := 0; i < count; i++ {
		row := table[4+i*24 : 4+(i+1)*24]
		addr := binary.LittleEndian.Uint32(row[4:])
		p := int(binary.BigEndian.Uint16(row[8:10]))
		if (addr == 0 || addr == 0x0100007f) && p == port && binary.LittleEndian.Uint32(row[20:]) == pid {
			return true
		}
	}
	return false
}

func tcp6OwnsPort(table []byte, port int, pid uint32) bool {
	if len(table) < 4 {
		return false
	}
	count := int(binary.LittleEndian.Uint32(table))
	if count > (len(table)-4)/56 {
		return false
	}
	for i := 0; i < count; i++ {
		row := table[4+i*56 : 4+(i+1)*56]
		unspecified := true
		for _, b := range row[:16] {
			if b != 0 {
				unspecified = false
				break
			}
		}
		if unspecified && int(binary.BigEndian.Uint16(row[20:22])) == port && binary.LittleEndian.Uint32(row[52:]) == pid {
			return true
		}
	}
	return false
}
func ServiceOwnsPort(port int) bool {
	pid := servicePID()
	if pid == 0 {
		return false
	}
	proc := syscall.NewLazyDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")
	for _, family := range []uintptr{2, 23} {
		var size uint32
		proc.Call(0, uintptr(unsafe.Pointer(&size)), 0, family, 3, 0)
		for tries := 0; tries < 3 && size >= 4 && size <= 16<<20; tries++ {
			data := make([]byte, size)
			rc, _, _ := proc.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0, family, 3, 0)
			if rc == 0 {
				if family == 2 && tcpOwnsPort(data, port, pid) || family == 23 && tcp6OwnsPort(data, port, pid) {
					return true
				}
				break
			}
			if rc != 122 {
				break
			}
		}
	}
	return false
}
