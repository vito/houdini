package process

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/contester/runlib/win32"
)

const PROCESS_ALL_ACCESS = 0x001F0FFF // im sure that's fine

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var procTerminateJobObject = kernel32.NewProc("TerminateJobObject")

func terminateJobObject(thread syscall.Handle, exitCode uint32) (err error) {
	if r1, _, e1 := procTerminateJobObject.Call(uintptr(thread), uintptr(exitCode)); int(r1) != 0 {
		return os.NewSyscallError("TerminateJobObject", e1)
	}
	return nil
}

func spawn(cmd *exec.Cmd) (process, error) {
	ro, wo, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pipe failed: %s", err)
	}

	re, we, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pipe failed: %s", err)
	}

	ri, wi, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pipe failed: %s", err)
	}

	go io.Copy(cmd.Stdout, ro)
	go io.Copy(cmd.Stderr, re)
	go io.Copy(wi, cmd.Stdin)

	attr := &syscall.ProcAttr{
		Dir:   cmd.Dir,
		Env:   cmd.Env,
		Files: []uintptr{ri.Fd(), wo.Fd(), we.Fd()},
	}

	lookedUpPath, err := lookExtensions(cmd.Path, cmd.Dir)
	if err != nil {
		return nil, fmt.Errorf("look extensions failed: %s", err)
	}

	// Acquire the fork lock so that no other threads
	// create new fds that are not yet close-on-exec
	// before we fork.
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()

	p, _ := syscall.GetCurrentProcess()
	fd := make([]syscall.Handle, len(attr.Files))
	for i := range attr.Files {
		if attr.Files[i] > 0 {
			err := syscall.DuplicateHandle(p, syscall.Handle(attr.Files[i]), p, &fd[i], 0, true, syscall.DUPLICATE_SAME_ACCESS)
			if err != nil {
				return nil, fmt.Errorf("duplicating handle failed: %s", err)
			}
			defer syscall.CloseHandle(syscall.Handle(fd[i]))
		}
	}

	si := new(syscall.StartupInfo)
	si.Cb = uint32(unsafe.Sizeof(*si))
	si.Flags = syscall.STARTF_USESTDHANDLES
	si.StdInput = fd[0]
	si.StdOutput = fd[1]
	si.StdErr = fd[2]

	pi := new(syscall.ProcessInformation)

	flags := uint32(syscall.CREATE_UNICODE_ENVIRONMENT)
	flags |= win32.CREATE_SUSPENDED
	flags |= win32.CREATE_BREAKAWAY_FROM_JOB

	argvp0, err := syscall.UTF16PtrFromString(lookedUpPath)
	if err != nil {
		return nil, fmt.Errorf("stringing failed: %s", err)
	}

	argvp0v0v0v0, err := syscall.UTF16PtrFromString(makeCmdLine(cmd.Args))
	if err != nil {
		return nil, fmt.Errorf("stringing failed: %s", err)
	}

	dirp, err := syscall.UTF16PtrFromString(attr.Dir)
	if err != nil {
		return nil, fmt.Errorf("stringing failed: %s", err)
	}

	err = syscall.CreateProcess(
		argvp0,
		argvp0v0v0v0,
		nil,
		nil,
		true,
		flags,
		createEnvBlock(attr.Env),
		dirp,
		si,
		pi,
	)
	if err != nil {
		return nil, fmt.Errorf("create process: %s", err)
	}

	jobName, err := syscall.UTF16PtrFromString(fmt.Sprintf("%d", time.Now().UnixNano()))
	if err != nil {
		return nil, fmt.Errorf("stringing failed: %s", err)
	}

	jobHandle, err := win32.CreateJobObject(nil, jobName)
	if err != nil {
		return nil, fmt.Errorf("create job failed: %s", err)
	}

	err = win32.AssignProcessToJobObject(jobHandle, pi.Process)
	if err != nil {
		return nil, fmt.Errorf("assign failed: %s", err)
	}

	_, err = win32.ResumeThread(pi.Thread)
	if err != nil {
		return nil, fmt.Errorf("resume failed: %s", err)
	}

	return &jobProcess{
		jobHandle:     jobHandle,
		processHandle: pi.Process,
	}, nil
}

type jobProcess struct {
	jobHandle     syscall.Handle
	processHandle syscall.Handle
}

func (process *jobProcess) Terminate() error {
	return terminateJobObject(process.jobHandle, 1)
}

func (process *jobProcess) Wait() (int, error) {
	s, e := syscall.WaitForSingleObject(syscall.Handle(process.processHandle), syscall.INFINITE)
	switch s {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		return -1, os.NewSyscallError("WaitForSingleObject", e)
	default:
		return -1, errors.New("os: unexpected result from WaitForSingleObject")
	}

	var ec uint32
	e = syscall.GetExitCodeProcess(syscall.Handle(process.processHandle), &ec)
	if e != nil {
		return -1, os.NewSyscallError("GetExitCodeProcess", e)
	}

	var u syscall.Rusage
	e = syscall.GetProcessTimes(syscall.Handle(process.processHandle), &u.CreationTime, &u.ExitTime, &u.KernelTime, &u.UserTime)
	if e != nil {
		return -1, os.NewSyscallError("GetProcessTimes", e)
	}

	// NOTE(brainman): It seems that sometimes process is not dead
	// when WaitForSingleObject returns. But we do not know any
	// other way to wait for it. Sleeping for a while seems to do
	// the trick sometimes. So we will sleep and smell the roses.
	defer time.Sleep(5 * time.Millisecond)
	defer syscall.CloseHandle(syscall.Handle(process.processHandle))

	return int(ec), nil
}

func makeCmdLine(args []string) string {
	var s string
	for _, v := range args {
		if s != "" {
			s += " "
		}
		s += syscall.EscapeArg(v)
	}
	return s
}

func createEnvBlock(envv []string) *uint16 {
	if len(envv) == 0 {
		return &utf16.Encode([]rune("\x00\x00"))[0]
	}
	length := 0
	for _, s := range envv {
		length += len(s) + 1
	}
	length += 1

	b := make([]byte, length)
	i := 0
	for _, s := range envv {
		l := len(s)
		copy(b[i:i+l], []byte(s))
		copy(b[i+l:i+l+1], []byte{0})
		i = i + l + 1
	}
	copy(b[i:i+1], []byte{0})

	return &utf16.Encode([]rune(string(b)))[0]
}

// lookExtensions finds windows executable by its dir and path.
// It uses LookPath to try appropriate extensions.
// lookExtensions does not search PATH, instead it converts `prog` into `.\prog`.
func lookExtensions(path, dir string) (string, error) {
	if filepath.Base(path) == path {
		path = filepath.Join(".", path)
	}
	if dir == "" {
		return exec.LookPath(path)
	}
	if filepath.VolumeName(path) != "" {
		return exec.LookPath(path)
	}
	if len(path) > 1 && os.IsPathSeparator(path[0]) {
		return exec.LookPath(path)
	}
	dirandpath := filepath.Join(dir, path)
	// We assume that LookPath will only add file extension.
	lp, err := exec.LookPath(dirandpath)
	if err != nil {
		return "", err
	}
	ext := strings.TrimPrefix(lp, dirandpath)
	return path + ext, nil
}
