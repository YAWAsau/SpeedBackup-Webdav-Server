package console

import (
	"os"
	"syscall"
	"unsafe"
)

func ioctl(f *os.File, op uintptr, p unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), op, uintptr(p))
	if e != 0 {
		return e
	}
	return nil
}
func hideInput(f *os.File) (func(), error) {
	var old syscall.Termios
	if e := ioctl(f, syscall.TCGETS, unsafe.Pointer(&old)); e != nil {
		return nil, e
	}
	next := old
	next.Lflag &^= syscall.ECHO | syscall.ECHONL
	if e := ioctl(f, syscall.TCSETS, unsafe.Pointer(&next)); e != nil {
		return nil, e
	}
	return func() { _ = ioctl(f, syscall.TCSETS, unsafe.Pointer(&old)) }, nil
}
func enableOutput(f *os.File) (func(), bool) {
	var t syscall.Termios
	e := ioctl(f, syscall.TCGETS, unsafe.Pointer(&t))
	return func() {}, e == nil && os.Getenv("TERM") != "dumb"
}
func dimensions(f *os.File) (int, int) {
	var size [4]uint16
	if ioctl(f, syscall.TIOCGWINSZ, unsafe.Pointer(&size)) != nil {
		return 80, 24
	}
	return int(size[1]), int(size[0])
}
