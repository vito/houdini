// +build !windows

package process

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/kr/pty"
	"github.com/vito/houdini/ptyutil"
)

func spawn(cmd *exec.Cmd, ttySpec *garden.TTYSpec, stdout io.Writer, stderr io.Writer) (process, io.WriteCloser, error) {
	var stdin io.WriteCloser
	var err error

	if ttySpec != nil {
		pty, tty, err := pty.Open()
		if err != nil {
			return nil, nil, err
		}

		stdin = pty

		windowColumns := 80
		windowRows := 24
		if ttySpec.WindowSize != nil {
			windowColumns = ttySpec.WindowSize.Columns
			windowRows = ttySpec.WindowSize.Rows
		}

		ptyutil.SetWinSize(pty, windowColumns, windowRows)

		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty

		go io.Copy(stdout, pty)
	} else {
		stdin, err = cmd.StdinPipe()
		if err != nil {
			return nil, nil, err
		}

		cmd.Stdout = stdout
		cmd.Stderr = stderr
	}

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	return &groupProcess{
		process: cmd.Process,
	}, stdin, nil
}

type groupProcess struct {
	process *os.Process
}

func (proc *groupProcess) Signal(signal garden.Signal) error {
	var err error

	switch signal {
	case garden.SignalTerminate:
		err = proc.process.Signal(syscall.SIGTERM)
	default: // only other case is kill, but if we don't know it, go nuclear
		err = proc.process.Signal(syscall.SIGKILL)
	}

	return err
}

func (proc *groupProcess) Wait() (int, error) {
	state, err := proc.process.Wait()
	if err != nil {
		return -1, err
	}

	return state.Sys().(syscall.WaitStatus).ExitStatus(), nil
}
