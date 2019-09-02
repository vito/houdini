package houdini

import (
	"context"
	"syscall"

	"code.cloudfoundry.org/garden"
	"github.com/containerd/containerd"
)

type process struct {
	t containerd.Task

	exitStatusC <-chan containerd.ExitStatus
}

func newProcess(t containerd.Task, exitStatusC <-chan containerd.ExitStatus) garden.Process {
	return &process{
		t: t,

		exitStatusC: exitStatusC,
	}
}

func (p *process) ID() string {
	return p.t.ID()
}

func (p *process) Wait() (int, error) {
	s := <-p.exitStatusC
	return int(s.ExitCode()), nil
}

func (p *process) SetTTY(spec garden.TTYSpec) error {
	if spec.WindowSize == nil {
		return nil
	}

	return p.t.Resize(
		context.TODO(),
		uint32(spec.WindowSize.Columns),
		uint32(spec.WindowSize.Rows),
	)
}

func (p *process) Signal(sig garden.Signal) error {
	s := syscall.SIGTERM
	if sig == garden.SignalKill {
		s = syscall.SIGKILL
	}

	return p.t.Kill(context.TODO(), s)
}
