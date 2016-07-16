package process

import (
	"os/exec"
	"sync"

	"code.cloudfoundry.org/garden"
)

type process interface {
	Signal(garden.Signal) error
	Wait() (int, error)
	SetWindowSize(garden.WindowSize) error
}

type Process struct {
	id string

	process process

	waiting    *sync.Once
	exitStatus int
	exitErr    error

	stdin  *faninWriter
	stdout *fanoutWriter
	stderr *fanoutWriter
}

func NewProcess(id string) *Process {
	return &Process{
		id: id,

		waiting: &sync.Once{},

		stdin:  &faninWriter{hasSink: make(chan struct{})},
		stdout: &fanoutWriter{},
		stderr: &fanoutWriter{},
	}
}

func (p *Process) ID() string {
	return p.id
}

func (p *Process) Wait() (int, error) {
	p.waiting.Do(func() {
		p.exitStatus, p.exitErr = p.process.Wait()

		// don't leak stdin pipe
		p.stdin.Close()
	})

	return p.exitStatus, p.exitErr
}

func (p *Process) SetTTY(tty garden.TTYSpec) error {
	if tty.WindowSize != nil {
		return p.process.SetWindowSize(*tty.WindowSize)
	}

	return nil
}

func (p *Process) Start(cmd *exec.Cmd, tty *garden.TTYSpec) error {
	process, stdin, err := spawn(cmd, tty, p.stdout, p.stderr)
	if err != nil {
		return err
	}

	p.stdin.AddSink(stdin)

	p.process = process

	return nil
}

func (p *Process) Attach(processIO garden.ProcessIO) {
	if processIO.Stdin != nil {
		p.stdin.AddSource(processIO.Stdin)
	}

	if processIO.Stdout != nil {
		p.stdout.AddSink(processIO.Stdout)
	}

	if processIO.Stderr != nil {
		p.stderr.AddSink(processIO.Stderr)
	}
}

func (p *Process) Signal(signal garden.Signal) error {
	return p.process.Signal(signal)
}
