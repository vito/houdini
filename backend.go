package houdini

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	garden "github.com/cloudfoundry-incubator/garden/api"
)

var (
	ErrContainerNotFound = errors.New("container not found")
)

type Backend struct {
	containersDir string
	skeletonDir   string

	containers  map[string]*container
	containersL sync.RWMutex

	containerNum uint64
}

func NewBackend(containersDir string, skeletonDir string) *Backend {
	return &Backend{
		containersDir: containersDir,
		skeletonDir:   skeletonDir,

		containers: make(map[string]*container),

		containerNum: uint64(time.Now().UnixNano()),
	}
}

func (backend *Backend) Start() error {
	return os.MkdirAll(backend.containersDir, 0755)
}

func (backend *Backend) Stop() {}

func (backend *Backend) GraceTime(garden.Container) time.Duration {
	return 5 * time.Minute
}

func (backend *Backend) Ping() error {
	return nil
}

func (backend *Backend) Capacity() (garden.Capacity, error) {
	println("NOT IMPLEMENTED: Capacity")
	return garden.Capacity{}, nil
}

func (backend *Backend) Create(spec garden.ContainerSpec) (garden.Container, error) {
	id := backend.generateContainerID()

	if spec.Handle == "" {
		spec.Handle = id
	}

	dir := filepath.Join(backend.containersDir, id)

	err := exec.Command("cp", "-a", backend.skeletonDir, dir).Run()
	// err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	container := newContainer(spec, dir)

	backend.containersL.Lock()
	backend.containers[spec.Handle] = container
	backend.containersL.Unlock()

	return container, nil
}

func (backend *Backend) Destroy(handle string) error {
	backend.containersL.RLock()
	container, found := backend.containers[handle]
	backend.containersL.RUnlock()

	if !found {
		return ErrContainerNotFound
	}

	err := container.Stop(false)
	if err != nil {
		return err
	}

	return os.RemoveAll(container.dir)
}

func (backend *Backend) Containers(filter garden.Properties) ([]garden.Container, error) {
	matchingContainers := []garden.Container{}

	backend.containersL.RLock()

	for _, container := range backend.containers {
		if containerHasProperties(container, filter) {
			matchingContainers = append(matchingContainers, container)
		}
	}

	backend.containersL.RUnlock()

	return matchingContainers, nil
}

func (backend *Backend) Lookup(handle string) (garden.Container, error) {
	backend.containersL.RLock()
	container, found := backend.containers[handle]
	backend.containersL.RUnlock()

	if !found {
		return nil, ErrContainerNotFound
	}

	return container, nil
}

func (backend *Backend) generateContainerID() string {
	containerNum := atomic.AddUint64(&backend.containerNum, 1)

	containerID := []byte{}

	var i uint64
	for i = 0; i < 11; i++ {
		containerID = strconv.AppendUint(
			containerID,
			(containerNum>>(55-(i+1)*5))&31,
			32,
		)
	}

	return string(containerID)
}

func containerHasProperties(container *container, properties garden.Properties) bool {
	containerProps := container.currentProperties()

	for key, val := range properties {
		cval, ok := containerProps[key]
		if !ok {
			return false
		}

		if cval != val {
			return false
		}
	}

	return true
}
