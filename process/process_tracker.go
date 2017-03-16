package process

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/nu7hatch/gouuid"
)

type ProcessTracker interface {
	Run(string, *exec.Cmd, garden.ProcessIO, *garden.TTYSpec) (garden.Process, error)
	Attach(string, garden.ProcessIO) (garden.Process, error)
	Restore(processID string)
	ActiveProcesses() []garden.Process
	Stop(kill bool) error
}

type processTracker struct {
	processes      map[string]*Process
	processesMutex *sync.RWMutex
}

type UnknownProcessError struct {
	ProcessID string
}

func (e UnknownProcessError) Error() string {
	return fmt.Sprintf("unknown process: %s", e.ProcessID)
}

func NewTracker() ProcessTracker {
	return &processTracker{
		processes:      make(map[string]*Process),
		processesMutex: new(sync.RWMutex),
	}
}

func (t *processTracker) Run(passedID string, cmd *exec.Cmd, processIO garden.ProcessIO, tty *garden.TTYSpec) (garden.Process, error) {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	processID := passedID
	if processID == "" {
		uuid, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}

		processID = uuid.String()
	}

	process := NewProcess(processID)

	process.Attach(processIO)

	err := process.Start(cmd, tty)
	if err != nil {
		return nil, err
	}

	t.processes[processID] = process

	return process, nil
}

func (t *processTracker) Attach(processID string, processIO garden.ProcessIO) (garden.Process, error) {
	t.processesMutex.RLock()
	process, ok := t.processes[processID]
	t.processesMutex.RUnlock()

	if !ok {
		return nil, UnknownProcessError{processID}
	}

	process.Attach(processIO)

	go t.waitAndReap(processID)

	return process, nil
}

func (t *processTracker) Restore(processID string) {
	t.processesMutex.Lock()

	process := NewProcess(processID)

	t.processes[processID] = process

	go t.waitAndReap(processID)

	t.processesMutex.Unlock()
}

func (t *processTracker) ActiveProcesses() []garden.Process {
	t.processesMutex.RLock()
	defer t.processesMutex.RUnlock()

	processes := make([]garden.Process, len(t.processes))

	i := 0
	for _, process := range t.processes {
		processes[i] = process
		i++
	}

	return processes
}

func (t *processTracker) Stop(kill bool) error {
	t.processesMutex.RLock()

	processes := make([]*Process, len(t.processes))

	i := 0
	for _, process := range t.processes {
		processes[i] = process
		i++
	}

	t.processesMutex.RUnlock()

	wait := new(sync.WaitGroup)
	wait.Add(len(processes))

	for _, process := range processes {
		exited := make(chan struct{})

		go func(process *Process) {
			process.Wait()
			close(exited)
			wait.Done()
		}(process)

		if kill {
			process.Signal(garden.SignalKill)
		} else {
			process.Signal(garden.SignalTerminate)

			go func(process *Process) {
				select {
				case <-exited:
				case <-time.After(10 * time.Second):
					process.Signal(garden.SignalKill)
				}
			}(process)
		}
	}

	wait.Wait()

	return nil
}

func (t *processTracker) waitAndReap(processID string) {
	t.processesMutex.RLock()
	process, ok := t.processes[processID]
	t.processesMutex.RUnlock()

	if !ok {
		return
	}

	process.Wait()

	t.unregister(processID)
}

func (t *processTracker) unregister(processID string) {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	delete(t.processes, processID)
}
