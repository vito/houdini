package ptyutil

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type ttySize struct {
	Rows   uint16
	Cols   uint16
	Xpixel uint16
	Ypixel uint16
}

func SetWinSize(f *os.File, cols int, rows int) error {
	return unix.IoctlSetInt(int(f.Fd()), int(syscall.TIOCSWINSZ),
		int(uintptr(unsafe.Pointer(&ttySize{uint16(rows), uint16(cols), 0, 0}))))
}
