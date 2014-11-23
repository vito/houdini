package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	linkpkg "github.com/cloudfoundry-incubator/garden-linux/old/iodaemon/link"
	"github.com/cloudfoundry-incubator/garden-linux/old/ptyutil"
	"github.com/kr/pty"
)

func spawn(socketPath string, argv []string, timeout time.Duration, withTty bool, windowColumns int, windowRows int, debug bool) {
	err := os.MkdirAll(filepath.Dir(socketPath), 0755)
	if err != nil {
		fatal(err)
	}

	if debug {
		ownPid := os.Getpid()

		traceOut, err := os.Create(socketPath + ".trace")
		if err != nil {
			fatal(err)
		}

		strace := exec.Command("strace", "-f", "-s", "10240", "-p", strconv.Itoa(ownPid))
		strace.Stdout = traceOut
		strace.Stderr = traceOut

		err = strace.Start()
		if err != nil {
			fatal(err)
		}
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fatal(err)
	}

	bin, err := exec.LookPath(argv[0])
	if err != nil {
		fatal(err)
	}

	cmd := child(bin, argv)

	// stderr will not be assigned in the case of a tty, so make
	// a dummy pipe to send across instead
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		fatal(err)
	}

	var stdinW, stdoutR *os.File
	var stdinR, stdoutW *os.File

	if withTty {
		pty, tty, err := pty.Open()
		if err != nil {
			fatal(err)
		}

		// do NOT assign stderrR to pty; the receiving end should only receive one
		// pty output stream, as they're both the same fd

		stdinW = pty
		stdoutR = pty

		stdinR = tty
		stdoutW = tty
		stderrW = tty

		ptyutil.SetWinSize(stdinW, windowColumns, windowRows)

		cmd.SysProcAttr.Setctty = true
		cmd.SysProcAttr.Setsid = true
	} else {
		stdinR, stdinW, err = os.Pipe()
		if err != nil {
			fatal(err)
		}

		stdoutR, stdoutW, err = os.Pipe()
		if err != nil {
			fatal(err)
		}
	}

	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	statusR, statusW, err := os.Pipe()
	if err != nil {
		fatal(err)
	}

	fmt.Println("ready")

	started := false

	for {
		conn, err := listener.Accept()
		if err != nil {
			fatal(err)
			break
		}

		rights := syscall.UnixRights(
			int(stdoutR.Fd()),
			int(stderrR.Fd()),
			int(statusR.Fd()),
		)

		_, _, err = conn.(*net.UnixConn).WriteMsgUnix([]byte{}, rights, nil)
		if err != nil {
			break
		}

		if !started {
			err := cmd.Start()
			if err != nil {
				fatal(err)
			}

			// close no longer relevant pipe ends
			// this closes tty 3 times but that's OK
			stdinR.Close()
			stdoutW.Close()
			stderrW.Close()

			fmt.Println("pid:", cmd.Process.Pid)

			go func() {
				cmd.Wait()

				if cmd.ProcessState != nil {
					fmt.Fprintf(statusW, "%d\n", cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus())
				}

				os.Exit(0)
			}()

			// detach from parent process
			os.Stdin.Close()
			os.Stdout.Close()
			os.Stderr.Close()

			started = true
		}

		decoder := gob.NewDecoder(conn)

		for {
			var input linkpkg.Input
			err := decoder.Decode(&input)
			if err != nil {
				break
			}

			if input.WindowSize != nil {
				ptyutil.SetWinSize(stdinW, input.WindowSize.Columns, input.WindowSize.Rows)
				cmd.Process.Signal(syscall.SIGWINCH)
			} else if input.EOF {
				err := stdinW.Close()
				if err != nil {
					conn.Close()
					break
				}
			} else {
				_, err := stdinW.Write(input.Data)
				if err != nil {
					conn.Close()
					break
				}
			}
		}
	}
}

func fatal(err error) {
	println("fatal: " + err.Error())
	os.Exit(1)
}
