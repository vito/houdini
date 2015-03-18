package houdini

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/vito/houdini/process_tracker"
)

type UndefinedPropertyError struct {
	Key string
}

func (err UndefinedPropertyError) Error() string {
	return fmt.Sprintf("property does not exist: %s", err.Key)
}

type container struct {
	handle string

	workDir string

	properties  garden.Properties
	propertiesL sync.RWMutex

	env []string

	processTracker process_tracker.ProcessTracker
}

func newContainer(spec garden.ContainerSpec, dir string) *container {
	properties := spec.Properties
	if properties == nil {
		properties = garden.Properties{}
	}

	return &container{
		handle: spec.Handle,

		workDir: dir,

		properties: properties,

		env: spec.Env,

		processTracker: process_tracker.New(dir),
	}
}

func (container *container) Handle() string {
	return container.handle
}

func (container *container) Stop(kill bool) error {
	return container.processTracker.Stop(kill)
}

func (container *container) Info() (garden.ContainerInfo, error) { return garden.ContainerInfo{}, nil }

func (container *container) StreamIn(dstPath string, tarStream io.Reader) error {
	finalDestination := filepath.Join(container.workDir, dstPath)

	err := os.MkdirAll(finalDestination, 0755)
	if err != nil {
		return err
	}

	tarCmd := exec.Command("tar", "xf", "-", "-C", finalDestination)
	tarCmd.Stdin = tarStream

	return tarCmd.Run()
}

func (container *container) StreamOut(srcPath string) (io.ReadCloser, error) {
	absoluteSource := filepath.Join(container.workDir, srcPath)

	workingDir := filepath.Dir(absoluteSource)
	compressArg := filepath.Base(absoluteSource)
	if strings.HasSuffix(srcPath, "/") {
		workingDir = absoluteSource
		compressArg = "."
	}

	tarCmd := exec.Command("tar", "cf", "-", compressArg, "-C", workingDir)

	out, err := tarCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = tarCmd.Start()
	if err != nil {
		return nil, err
	}

	go tarCmd.Wait()

	return out, nil
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

func (container *container) Run(spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	spec.Path = spec.Path

	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = filepath.Join(container.workDir, spec.Dir)
	cmd.Env = append(os.Environ(), append(container.env, spec.Env...)...)

	return container.processTracker.Run(cmd, processIO, spec.TTY)
}

func (container *container) Attach(processID uint32, processIO garden.ProcessIO) (garden.Process, error) {
	return container.processTracker.Attach(processID, processIO)
}

func (container *container) GetProperty(name string) (string, error) {
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

func (container *container) GetProperties() (garden.Properties, error) {
	return container.currentProperties(), nil
}

func (container *container) Metrics() (garden.Metrics, error) {
	return garden.Metrics{}, nil
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
