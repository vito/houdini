package ptyutil

import (
	"os"

	"github.com/pkg/term/termios"
)

func SetRaw(tty *os.File) error {
	attr, err := termios.Tcgetattr(tty.Fd())
	if err != nil {
		return err
	}

	termios.Cfmakeraw(attr)

	return termios.Tcsetattr(tty.Fd(), termios.TCSANOW, attr)
}
