package houdini

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/charlievieth/fs"
	"github.com/concourse/go-archive/tarfs"
	"github.com/vito/houdini/process"
)

type UndefinedPropertyError struct {
	Key string
}

func (err UndefinedPropertyError) Error() string {
	return fmt.Sprintf("property does not exist: %s", err.Key)
}

type container struct {
	spec garden.ContainerSpec

	handle string

	workDir string

	properties  garden.Properties
	propertiesL sync.RWMutex

	env []string

	processTracker process.ProcessTracker

	graceTime  time.Duration
	graceTimeL sync.RWMutex
}

func newContainer(spec garden.ContainerSpec, workDir string) *container {
	properties := spec.Properties
	if properties == nil {
		properties = garden.Properties{}
	}

	return &container{
		spec: spec,

		handle: spec.Handle,

		workDir: workDir,

		properties: properties,

		env: spec.Env,

		processTracker: process.NewTracker(),
	}
}

func (container *container) Handle() string {
	return container.handle
}

func (container *container) Stop(kill bool) error {
	return container.processTracker.Stop(kill)
}

func (container *container) Info() (garden.ContainerInfo, error) { return garden.ContainerInfo{}, nil }

func (container *container) StreamIn(spec garden.StreamInSpec) error {
	finalDestination := filepath.Join(container.workDir, filepath.FromSlash(spec.Path))

	err := fs.MkdirAll(finalDestination, 0755)
	if err != nil {
		return err
	}

	err = tarfs.Extract(spec.TarStream, finalDestination)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	if strings.HasSuffix(spec.Path, "/") {
		spec.Path += "."
	}

	absoluteSource := container.workDir + string(os.PathSeparator) + filepath.FromSlash(spec.Path)

	r, w := io.Pipe()

	errs := make(chan error, 1)
	go func() {
		errs <- tarfs.Compress(w, filepath.Dir(absoluteSource), filepath.Base(absoluteSource))
		_ = w.Close()
	}()

	return waitCloser{
		ReadCloser: r,
		wait:       errs,
	}, nil
}

type waitCloser struct {
	io.ReadCloser
	wait <-chan error
}

func (c waitCloser) Close() error {
	err := c.ReadCloser.Close()
	if err != nil {
		return err
	}

	return <-c.wait
}

func (container *container) LimitBandwidth(limits garden.BandwidthLimits) error { return nil }

func (container *container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return garden.BandwidthLimits{}, nil
}

func (container *container) LimitCPU(limits garden.CPULimits) error { return nil }

func (container *container) CurrentCPULimits() (garden.CPULimits, error) {
	return garden.CPULimits{}, nil
}

func (container *container) LimitDisk(limits garden.DiskLimits) error { return nil }

func (container *container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return garden.DiskLimits{}, nil
}

func (container *container) LimitMemory(limits garden.MemoryLimits) error { return nil }

func (container *container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return garden.MemoryLimits{}, nil
}

func (container *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, nil
}

func (container *container) NetOut(garden.NetOutRule) error { return nil }

func (container *container) BulkNetOut([]garden.NetOutRule) error { return nil }

func (container *container) Run(spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	cmd := exec.Command(filepath.FromSlash(spec.Path), spec.Args...)
	cmd.Dir = filepath.Join(container.workDir, filepath.FromSlash(spec.Dir))
	cmd.Env = append(os.Environ(), append(container.env, spec.Env...)...)
	if spec.User != "" {
		if err := setUser(cmd, spec); err != nil {
			return nil, err
		}
	}

	return container.processTracker.Run(spec.ID, cmd, processIO, spec.TTY)
}

func (container *container) Attach(processID string, processIO garden.ProcessIO) (garden.Process, error) {
	return container.processTracker.Attach(processID, processIO)
}

func (container *container) Property(name string) (string, error) {
	container.propertiesL.RLock()
	property, found := container.properties[name]
	container.propertiesL.RUnlock()

	if !found {
		return "", UndefinedPropertyError{name}
	}

	return property, nil
}

func (container *container) SetProperty(name string, value string) error {
	container.propertiesL.Lock()
	container.properties[name] = value
	container.propertiesL.Unlock()

	return nil
}

func (container *container) RemoveProperty(name string) error {
	container.propertiesL.Lock()
	defer container.propertiesL.Unlock()

	_, found := container.properties[name]
	if !found {
		return UndefinedPropertyError{name}
	}

	delete(container.properties, name)

	return nil
}

func (container *container) Properties() (garden.Properties, error) {
	return container.currentProperties(), nil
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

func (container *container) currentProperties() garden.Properties {
	properties := garden.Properties{}

	container.propertiesL.RLock()

	for k, v := range container.properties {
		properties[k] = v
	}

	container.propertiesL.RUnlock()

	return properties
}

func (container *container) currentGraceTime() time.Duration {
	container.graceTimeL.RLock()
	defer container.graceTimeL.RUnlock()
	return container.graceTime
}
