package main

import (
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/kr/pty"
	linkpkg "github.com/vito/houdini/iodaemon/link"
)

func link(socketPath string) {
	l, err := linkpkg.Create(socketPath, os.Stdout, os.Stderr)
	if err != nil {
		fatal(err)
	}

	resized := make(chan os.Signal, 10)

	go func() {
		for {
			<-resized

			rows, cols, err := pty.Getsize(os.Stdin)
			if err == nil {
				l.SetWindowSize(cols, rows)
			}
		}
	}()

	signal.Notify(resized, syscall.SIGWINCH)

	go io.Copy(l, os.Stdin)

	status, err := l.Wait()
	if err != nil {
		fatal(err)
	}

	os.Exit(status)
}
