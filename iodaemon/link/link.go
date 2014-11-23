package link

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
)

type Link struct {
	*Writer

	exitStatus io.Reader
	streaming  *sync.WaitGroup
}

func Create(socketPath string, stdout io.Writer, stderr io.Writer) (*Link, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to i/o daemon: %s", err)
	}

	var b [2048]byte
	var oob [2048]byte

	n, oobn, _, _, err := conn.(*net.UnixConn).ReadMsgUnix(b[:], oob[:])
	if err != nil {
		return nil, fmt.Errorf("failed to read unix msg: %s (read: %d, %d)", err, n, oobn)
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return nil, fmt.Errorf("failed to parse socket control message: %s", err)
	}

	if len(scms) < 1 {
		return nil, fmt.Errorf("no socket control messages sent")
	}

	scm := scms[0]

	fds, err := syscall.ParseUnixRights(&scm)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unix rights: %s", err)
	}

	if len(fds) != 3 {
		return nil, fmt.Errorf("invalid number of fds; need 3, got %d", len(fds))
	}

	lstdout := os.NewFile(uintptr(fds[0]), "stdout")
	lstderr := os.NewFile(uintptr(fds[1]), "stderr")
	lstatus := os.NewFile(uintptr(fds[2]), "status")

	streaming := &sync.WaitGroup{}

	linkWriter := NewWriter(conn)

	streaming.Add(1)
	go func() {
		io.Copy(stdout, lstdout)
		lstdout.Close()
		streaming.Done()
	}()

	streaming.Add(1)
	go func() {
		io.Copy(stderr, lstderr)
		lstderr.Close()
		streaming.Done()
	}()

	return &Link{
		Writer: linkWriter,

		exitStatus: lstatus,
		streaming:  streaming,
	}, nil
}

func (link *Link) Wait() (int, error) {
	link.streaming.Wait()

	var exitStatus int
	_, err := fmt.Fscanf(link.exitStatus, "%d\n", &exitStatus)
	if err != nil {
		return -1, fmt.Errorf("could not determine exit status: %s", err)
	}

	return exitStatus, nil
}
