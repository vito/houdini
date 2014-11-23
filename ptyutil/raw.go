package ptyutil

import (
	"os"
	"syscall"

	"github.com/pkg/term/termios"
)

func SetRaw(tty *os.File) error {
	var attr syscall.Termios

	err := termios.Tcgetattr(uintptr(tty.Fd()), (*syscall.Termios)(&attr))
	if err != nil {
		return err
	}

	termios.Cfmakeraw((*syscall.Termios)(&attr))

	return termios.Tcsetattr(uintptr(tty.Fd()), termios.TCSANOW, (*syscall.Termios)(&attr))
}
