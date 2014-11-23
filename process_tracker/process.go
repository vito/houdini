package process_tracker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"

	"github.com/cloudfoundry-incubator/garden/api"
	"github.com/vito/houdini/iodaemon/link"
)

type Process struct {
	id uint32

	containerPath string

	runningLink *sync.Once

	linked chan struct{}
	link   *link.Link

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

func (p *Process) SetTTY(tty api.TTYSpec) error {
	<-p.linked

	if tty.WindowSize != nil {
		return p.link.SetWindowSize(tty.WindowSize.Columns, tty.WindowSize.Rows)
	}

	return nil
}

func (p *Process) Spawn(cmd *exec.Cmd, tty *api.TTYSpec) (ready, active chan error) {
	ready = make(chan error, 1)
	active = make(chan error, 1)

	spawnPath := path.Join(p.containerPath, "bin", "iodaemon")
	processSock := path.Join(p.containerPath, "processes", fmt.Sprintf("%d.sock", p.ID()))

	spawnFlags := []string{}

	if tty != nil {
		spawnFlags = append(spawnFlags, "-tty")

		if tty.WindowSize != nil {
			spawnFlags = append(
				spawnFlags,
				fmt.Sprintf("-windowColumns=%d", tty.WindowSize.Columns),
				fmt.Sprintf("-windowRows=%d", tty.WindowSize.Rows),
			)
		}
	}

	spawnFlags = append(spawnFlags, "spawn", processSock)

	spawn := exec.Command(spawnPath, append(spawnFlags, cmd.Args...)...)
	spawn.Env = cmd.Env
	spawn.Dir = cmd.Dir
	spawn.Stderr = os.Stderr
	spawn.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	spawnR, err := spawn.StdoutPipe()
	if err != nil {
		ready <- err
		return
	}

	spawnOut := bufio.NewReader(spawnR)

	err = spawn.Start()
	if err != nil {
		ready <- err
		return
	}

	go func() {
		defer spawn.Wait()

		_, err := spawnOut.ReadBytes('\n')
		if err != nil {
			ready <- fmt.Errorf("failed to read ready: %s", err)
			return
		}

		ready <- nil

		_, err = spawnOut.ReadBytes('\n')
		if err != nil {
			active <- fmt.Errorf("failed to read active: %s", err)
			return
		}

		active <- nil
	}()

	return
}

func (p *Process) Link() {
	p.runningLink.Do(p.runLinker)
}

func (p *Process) Attach(processIO api.ProcessIO) {
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

func (p *Process) Signal(signal os.Signal) error {
	return p.link.SendSignal(signal)
}

func (p *Process) runLinker() {
	processSock := path.Join(p.containerPath, "processes", fmt.Sprintf("%d.sock", p.ID()))

	link, err := link.Create(processSock, p.stdout, p.stderr)
	if err != nil {
		p.completed(-1, err)
		return
	}

	p.stdin.AddSink(link)

	p.link = link
	close(p.linked)

	p.completed(p.link.Wait())

	// don't leak stdin pipe
	p.stdin.Close()
}

func (p *Process) completed(exitStatus int, err error) {
	p.exitStatus = exitStatus
	p.exitErr = err
	close(p.exited)
}
