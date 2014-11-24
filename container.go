package houdini

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	garden "github.com/cloudfoundry-incubator/garden/api"
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

	dir     string
	workDir string
	tmpDir  string

	properties  garden.Properties
	propertiesL sync.RWMutex

	env []string

	processTracker process_tracker.ProcessTracker
}

func newContainer(spec garden.ContainerSpec, dir string) *container {
	workDir := filepath.Join(dir, "workdir")
	tmpDir := filepath.Join(dir, "tmpdir")

	return &container{
		handle: spec.Handle,

		dir:     dir,
		workDir: workDir,
		tmpDir:  tmpDir,

		properties: spec.Properties,

		env: append(
			spec.Env,
			"PATH=/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin",
			"TMPDIR="+tmpDir,
			"HOME="+workDir,
		),

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

func (container *container) NetOut(network string, port uint32) error { return nil }

func (container *container) Run(spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	args := []string{
		"-p",
		fmt.Sprintf(`
		(version 1)
		(deny default)
		(debug deny)
		(allow network*)
		(allow process*)
		(allow signal (target self))
		(allow mach-lookup)
		(allow sysctl-read)
		(allow ipc*)
		(allow file-read*)
		(deny file-read* (subpath %q))
		(allow file-read* (subpath %q))
		(allow file* (subpath %q) (subpath %q))
		`, filepath.Dir(container.dir), container.dir, container.workDir, container.tmpDir),
		spec.Path,
	}

	args = append(args, spec.Args...)

	cmd := exec.Command("sandbox-exec", args...)
	cmd.Dir = filepath.Join(container.workDir, spec.Dir)
	cmd.Env = append(container.env, spec.Env...)

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

func (container *container) currentProperties() garden.Properties {
	properties := garden.Properties{}

	container.propertiesL.RLock()

	for k, v := range container.properties {
		properties[k] = v
	}

	container.propertiesL.RUnlock()

	return properties
}
