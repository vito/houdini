package process

import (
	"os/exec"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
)

type process interface {
	Signal(garden.Signal) error
	Wait() (int, error)
}

type Process struct {
	id uint32

	runningLink *sync.Once

	linked  chan struct{}
	process process

	exited     chan struct{}
	exitStatus int
	exitErr    error

	stdin  *faninWriter
	stdout *fanoutWriter
	stderr *fanoutWriter
}

func NewProcess(id uint32) *Process {
	return &Process{
		id: id,

		runningLink: &sync.Once{},

		linked: make(chan struct{}),

		exited: make(chan struct{}),

		stdin:  &faninWriter{hasSink: make(chan struct{})},
		stdout: &fanoutWriter{},
		stderr: &fanoutWriter{},
	}
}

func (p *Process) ID() uint32 {
	return p.id
}

func (p *Process) Wait() (int, error) {
	<-p.exited
	return p.exitStatus, p.exitErr
}

func (p *Process) SetTTY(tty garden.TTYSpec) error {
	<-p.linked

	// if tty.WindowSize != nil {
	// 	return p.link.SetWindowSize(tty.WindowSize.Columns, tty.WindowSize.Rows)
	// }

	return nil
}

func (p *Process) Spawn(cmd *exec.Cmd, tty *garden.TTYSpec) (ready, active chan error) {
	ready = make(chan error, 1)
	active = make(chan error, 1)

	process, stdin, err := spawn(cmd, tty, p.stdout, p.stderr)
	if err != nil {
		ready <- err
		return
	}

	p.stdin.AddSink(stdin)

	p.process = process

	ready <- nil
	active <- nil

	return
}

func (p *Process) Link() {
	p.runningLink.Do(p.runLinker)
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
	select {
	case <-p.linked:
		return p.process.Signal(signal)
	default:
		return nil
	}
}

func (p *Process) runLinker() {
	close(p.linked)

	p.completed(p.process.Wait())

	// don't leak stdin pipe
	p.stdin.Close()
}

func (p *Process) completed(exitStatus int, err error) {
	p.exitStatus = exitStatus
	p.exitErr = err
	close(p.exited)
}
