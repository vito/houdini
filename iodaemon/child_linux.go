package main

import (
	"os/exec"
	"syscall"
)

func child(bin string, argv []string) *exec.Cmd {
	return &exec.Cmd{
		Path: bin,
		Args: argv,
		SysProcAttr: &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGKILL,
		},
	}
}
