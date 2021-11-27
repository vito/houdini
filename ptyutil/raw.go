package ptyutil

import (
	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
	"os"
)

func SetRaw(tty *os.File) error {
	var attr unix.Termios

	err := termios.Tcgetattr(tty.Fd(), &attr)
	if err != nil {
		return err
	}

	termios.Cfmakeraw(&attr)

	return termios.Tcsetattr(tty.Fd(), termios.TCSANOW, &attr)
}
