package process_tracker

import (
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
)

type Process struct {
	id uint32

	containerPath string

	runningLink *sync.Once

	linked chan struct{}
	link   *exec.Cmd

	exited     chan struct{}
	exitStatus int
	exitErr    error

	stdin  *faninWriter
	stdout *fanoutWriter
	stderr *fanoutWriter
}

func NewProcess(id uint32, containerPath string) *Process {
	return &Process{
		id: id,

		containerPath: containerPath,

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

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		ready <- err
		return
	}

	p.stdin.AddSink(stdinPipe)

	cmd.Stdout = p.stdout
	cmd.Stderr = p.stderr

	err = cmd.Start()
	if err != nil {
		ready <- err
		return
	}

	p.link = cmd

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
		return p.link.Process.Signal(os.Kill)
	default:
		return nil
	}
}

func (p *Process) runLinker() {
	close(p.linked)

	status, err := p.link.Process.Wait()
	if err != nil {
		p.completed(-1, err)
	} else {
		p.completed(status.Sys().(syscall.WaitStatus).ExitStatus(), nil)
	}

	// don't leak stdin pipe
	p.stdin.Close()
}

func (p *Process) completed(exitStatus int, err error) {
	p.exitStatus = exitStatus
	p.exitErr = err
	close(p.exited)
}
