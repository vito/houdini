package houdini

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/go-cni"
	"github.com/pkg/errors"
)

var ErrNotImplemented = errors.New("not implemented")

type UndefinedPropertyError struct {
	Key string
}

func (err UndefinedPropertyError) Error() string {
	return fmt.Sprintf("property does not exist: %s", err.Key)
}

type container struct {
	c containerd.Container
	n cni.CNI

	taskOpts interface{}

	graceTime  time.Duration
	graceTimeL sync.Mutex
}

func (container *container) Handle() string {
	return container.c.ID()
}

func (container *container) Stop(kill bool) error {
	return container.Stop(kill)
}

func (container *container) Info() (garden.ContainerInfo, error) {
	return garden.ContainerInfo{}, ErrNotImplemented
}

func (container *container) StreamIn(spec garden.StreamInSpec) error {
	return ErrNotImplemented
}

func (container *container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	return nil, ErrNotImplemented
	// if strings.HasSuffix(spec.Path, "/") {
	// 	spec.Path += "."
	// }

	// absoluteSource := container.workDir + string(os.PathSeparator) + filepath.FromSlash(spec.Path)

	// r, w := io.Pipe()

	// errs := make(chan error, 1)
	// go func() {
	// 	errs <- tarfs.Compress(w, filepath.Dir(absoluteSource), filepath.Base(absoluteSource))
	// 	_ = w.Close()
	// }()

	// return waitCloser{
	// 	ReadCloser: r,
	// 	wait:       errs,
	// }, nil
}

// type waitCloser struct {
// 	io.ReadCloser
// 	wait <-chan error
// }

// func (c waitCloser) Close() error {
// 	err := c.ReadCloser.Close()
// 	if err != nil {
// 		return err
// 	}

// 	return <-c.wait
// }

func (container *container) LimitBandwidth(limits garden.BandwidthLimits) error {
	return ErrNotImplemented
}

func (container *container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return garden.BandwidthLimits{}, ErrNotImplemented
}

func (container *container) LimitCPU(limits garden.CPULimits) error {
	return ErrNotImplemented
}

func (container *container) CurrentCPULimits() (garden.CPULimits, error) {
	return garden.CPULimits{}, ErrNotImplemented
}

func (container *container) LimitDisk(limits garden.DiskLimits) error {
	return ErrNotImplemented
}

func (container *container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return garden.DiskLimits{}, ErrNotImplemented
}

func (container *container) LimitMemory(limits garden.MemoryLimits) error {
	return ErrNotImplemented
}

func (container *container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return garden.MemoryLimits{}, ErrNotImplemented
}

func (container *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, ErrNotImplemented
}

func (container *container) NetOut(garden.NetOutRule) error {
	return ErrNotImplemented
}

func (container *container) BulkNetOut([]garden.NetOutRule) error {
	return ErrNotImplemented
}

func (container *container) Run(spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	ctx := context.Background()

	streams := cio.WithStreams(
		processIO.Stdin,
		processIO.Stdout,
		processIO.Stderr,
	)

	task, err := container.c.NewTask(ctx, cio.NewCreator(streams), container.ioOpts)
	if err != nil {
		return nil, errors.Wrap(err, "new task")
	}
	defer task.Delete(ctx)

	netNs := fmt.Sprintf("/proc/%d/ns/net", task.Pid())
	netName := container.Handle()

	_, err = container.n.Setup(ctx, netName, netNs)
	if err != nil {
		return nil, errors.Wrap(err, "setup network")
	}

	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: Exec?
	if err := task.Start(ctx); err != nil {
		return nil, err
	}

	return newProcess(task, exitStatusC), nil
}

func (container *container) Attach(processID string, processIO garden.ProcessIO) (garden.Process, error) {
	return nil, ErrNotImplemented
}

func (container *container) Property(name string) (string, error) {
	labels, err := container.c.Labels(context.TODO())
	if err != nil {
		return "", err
	}

	val, found := labels[name]
	if !found {
		return "", UndefinedPropertyError{name}
	}

	return val, nil
}

func (container *container) SetProperty(name string, value string) error {
	_, err := container.c.SetLabels(context.TODO(), map[string]string{
		name: value,
	})
	return err
}

func (container *container) RemoveProperty(name string) error {
	return ErrNotImplemented
}

func (container *container) Properties() (garden.Properties, error) {
	return container.c.Labels(context.TODO())
}

func (container *container) Metrics() (garden.Metrics, error) {
	return garden.Metrics{}, nil
}

func (container *container) SetGraceTime(t time.Duration) error {
	container.graceTimeL.Lock()
	container.graceTime = t
	container.graceTimeL.Unlock()
	return nil
}

func (container *container) ioOpts(_ context.Context, client *containerd.Client, r *containerd.TaskInfo) error {
	r.Options = container.taskOpts
	return nil
}

func (container *container) currentGraceTime() time.Duration {
	container.graceTimeL.Lock()
	defer container.graceTimeL.Unlock()
	return container.graceTime
}

func (container *container) cleanup() error {
	ctx := context.Background()

	err := container.n.Remove(ctx, container.Handle(), "")
	if err != nil {
		return errors.Wrap(err, "remove network")
	}

	err = container.c.Delete(ctx, containerd.WithSnapshotCleanup)
	if err != nil {
		return errors.Wrap(err, "delete container")
	}

	return nil
}
