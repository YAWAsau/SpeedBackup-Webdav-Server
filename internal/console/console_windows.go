package console

import (
	"os"
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var getMode = kernel.NewProc("GetConsoleMode")
var setMode = kernel.NewProc("SetConsoleMode")
var screenInfo = kernel.NewProc("GetConsoleScreenBufferInfo")

func mode(f *os.File) (uint32, error) {
	var m uint32
	r, _, e := getMode.Call(f.Fd(), uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return 0, e
	}
	return m, nil
}
func change(f *os.File, m uint32) error {
	r, _, e := setMode.Call(f.Fd(), uintptr(m))
	if r == 0 {
		return e
	}
	return nil
}
func hideInput(f *os.File) (func(), error) {
	m, e := mode(f)
	if e != nil {
		return nil, e
	}
	if e = change(f, m&^4); e != nil {
		return nil, e
	}
	return func() { _ = change(f, m) }, nil
}
func enableOutput(f *os.File) (func(), bool) {
	m, e := mode(f)
	if e != nil {
		return func() {}, false
	}
	if change(f, m|4) != nil {
		return func() {}, false
	}
	return func() { _ = change(f, m) }, true
}
func dimensions(f *os.File) (int, int) {
	var info struct {
		Size       [2]int16
		Cursor     [2]int16
		Attributes uint16
		Window     [4]int16
		Maximum    [2]int16
	}
	r, _, _ := screenInfo.Call(f.Fd(), uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 80, 24
	}
	return int(info.Window[2] - info.Window[0] + 1), int(info.Window[3] - info.Window[1] + 1)
}
