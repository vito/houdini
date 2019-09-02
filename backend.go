package houdini

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/runtime/linux/runctypes"
	"github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/containerd/go-cni"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Backend struct {
	client    *containerd.Client
	namespace string

	network cni.CNI

	maxUid uint32
	maxGid uint32
}

func NewBackend(client *containerd.Client, namespace string) (*Backend, error) {
	maxUid, err := defaultUIDMap.MaxValid()
	if err != nil {
		return nil, err
	}

	maxGid, err := defaultGIDMap.MaxValid()
	if err != nil {
		return nil, err
	}

	network, err := cni.New(
		cni.WithPluginDir([]string{"plugins"}),
		cni.WithConfListFile("network.json"),
	)
	if err != nil {
		return nil, err
	}

	return &Backend{
		client:    client,
		namespace: namespace,

		network: network,

		maxUid: maxUid,
		maxGid: maxGid,
	}, nil
}

func (backend *Backend) Start() error {
	return nil
}

func (backend *Backend) Stop() {}

func (backend *Backend) GraceTime(c garden.Container) time.Duration {
	return c.(*container).currentGraceTime()
}

func (backend *Backend) Ping() error {
	// XXX: ping containerd?
	return nil
}

func (backend *Backend) Capacity() (garden.Capacity, error) {
	println("NOT IMPLEMENTED: Capacity")
	return garden.Capacity{}, nil
}

func (backend *Backend) Create(spec garden.ContainerSpec) (garden.Container, error) {
	client := backend.client

	ctx := namespaces.WithNamespace(context.Background(), "concourse")

	var image containerd.Image
	rootfsURI, err := url.Parse(spec.RootFSPath)
	if err != nil {
		return nil, err
	}

	switch rootfsURI.Scheme {
	case "oci":
		image, err = importImage(ctx, client, rootfsURI.Path)
	case "docker":
		image, err = client.Pull(ctx, rootfsURI.Host+rootfsURI.Path+":"+rootfsURI.Fragment, containerd.WithPullUnpack)
	default:
		return nil, fmt.Errorf("unknown rootfs uri: %s", spec.RootFSPath)
	}
	if err != nil {
		return nil, err
	}

	// XXX
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	mounts := []specs.Mount{
		{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"ro", "nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      filepath.Join(cwd, "etc", "resolv.conf"),
			Options:     []string{"rbind", "ro"},
		},
		{
			Destination: "/etc/hosts",
			Type:        "bind",
			Source:      filepath.Join(cwd, "etc", "hosts"),
			Options:     []string{"rbind", "ro"},
		},
	}

	for _, m := range spec.BindMounts {
		mount := specs.Mount{
			Destination: m.DstPath,
			Source:      m.SrcPath,
			Type:        "bind",
		}

		if m.Mode == garden.BindMountModeRO {
			mount.Options = []string{"ro"}
		}

		mounts = append(mounts, mount)
	}

	cont, err := client.NewContainer(
		ctx,
		spec.Handle,
		containerd.WithContainerLabels(spec.Properties),
		withRemappedSnapshotBase(spec.Handle, image, backend.maxUid, backend.maxGid, false),
		containerd.WithNewSpec(
			// inherit image config
			oci.WithImageConfig(image),

			// propagate env
			oci.WithEnv(spec.Env),

			// carry over garden defaults
			oci.WithDefaultUnixDevices,
			oci.WithLinuxDevice("/dev/fuse", "rwm"),

			// minimum required caps for running buildkit
			oci.WithAddedCapabilities([]string{
				"CAP_SYS_ADMIN",
				"CAP_NET_ADMIN",
			}),

			// enable user namespaces
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.UserNamespace,
			}),
			withRemappedRoot(backend.maxUid, backend.maxGid),

			// ...just set a hostname
			oci.WithHostname(spec.Handle),

			// wire up concourse stuff
			oci.WithMounts(mounts),
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "new container")
	}

	return backend.newContainer(cont)
}

func (backend *Backend) newContainer(cont containerd.Container) (garden.Container, error) {
	info, err := cont.Info(context.TODO())
	if err != nil {
		return nil, err
	}

	var taskOpts interface{}
	if containerd.CheckRuntime(info.Runtime.Name, "io.containerd.runc") {
		taskOpts = &options.Options{
			IoUid: backend.maxUid,
			IoGid: backend.maxGid,
		}
	} else {
		taskOpts = &runctypes.CreateOptions{
			IoUid: backend.maxUid,
			IoGid: backend.maxGid,
		}
	}

	return &container{
		c: cont,
		n: backend.network,

		taskOpts: taskOpts,
	}, nil
}

func (backend *Backend) Destroy(handle string) error {
	c, err := backend.Lookup(handle)
	if err != nil {
		return err
	}

	return c.(*container).cleanup()
}

func (backend *Backend) Containers(filter garden.Properties) ([]garden.Container, error) {
	return nil, nil
}

func (backend *Backend) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return map[string]garden.ContainerInfoEntry{}, nil
}

func (backend *Backend) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return map[string]garden.ContainerMetricsEntry{}, nil
}

func (backend *Backend) Lookup(handle string) (garden.Container, error) {
	ctx := context.Background()

	cont, err := backend.client.LoadContainer(ctx, handle)
	if err != nil {
		return nil, err
	}

	return backend.newContainer(cont)
}

func withRemappedRoot(maxUid, maxGid uint32) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *oci.Spec) error {
		s.Linux.UIDMappings = []specs.LinuxIDMapping{
			{
				ContainerID: 0,
				HostID:      maxUid,
				Size:        1,
			},
			{
				ContainerID: 1,
				HostID:      1,
				Size:        maxUid - 1,
			},
		}

		s.Linux.GIDMappings = []specs.LinuxIDMapping{
			{
				ContainerID: 0,
				HostID:      maxGid,
				Size:        1,
			},
			{
				ContainerID: 1,
				HostID:      1,
				Size:        maxGid - 1,
			},
		}

		return nil
	}
}
func withRemappedSnapshotBase(id string, i containerd.Image, uid, gid uint32, readonly bool) containerd.NewContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		diffIDs, err := i.RootFS(ctx)
		if err != nil {
			return err
		}

		var (
			parent   = identity.ChainID(diffIDs).String()
			usernsID = fmt.Sprintf("%s-%d-%d", parent, uid, gid)
		)

		c.Snapshotter, err = resolveSnapshotterName(client, ctx, c.Snapshotter)
		if err != nil {
			return err
		}

		snapshotter := client.SnapshotService(c.Snapshotter)

		if _, err := snapshotter.Stat(ctx, usernsID); err == nil {
			if _, err := snapshotter.Prepare(ctx, id, usernsID); err == nil {
				c.SnapshotKey = id
				c.Image = i.Name()
				return nil
			} else if !errdefs.IsNotFound(err) {
				return err
			}
		}

		mounts, err := snapshotter.Prepare(ctx, usernsID+"-remap", parent)
		if err != nil {
			return err
		}
		if err := remapRootFS(ctx, mounts, uid, gid); err != nil {
			snapshotter.Remove(ctx, usernsID)
			return err
		}
		if err := snapshotter.Commit(ctx, usernsID, usernsID+"-remap"); err != nil {
			return err
		}
		if readonly {
			_, err = snapshotter.View(ctx, id, usernsID)
		} else {
			_, err = snapshotter.Prepare(ctx, id, usernsID)
		}
		if err != nil {
			return err
		}
		c.SnapshotKey = id
		c.Image = i.Name()
		return nil
	}
}

func resolveSnapshotterName(c *containerd.Client, ctx context.Context, name string) (string, error) {
	if name == "" {
		label, err := c.GetLabel(ctx, defaults.DefaultSnapshotterNSLabel)
		if err != nil {
			return "", err
		}

		if label != "" {
			name = label
		} else {
			name = containerd.DefaultSnapshotter
		}
	}

	return name, nil
}

func remapRootFS(ctx context.Context, mounts []mount.Mount, uid, gid uint32) error {
	return mount.WithTempMount(ctx, mounts, func(root string) error {
		return filepath.Walk(root, remapRoot(root, uid, gid))
	})
}

func remapRoot(root string, toUid, toGid uint32) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		stat := info.Sys().(*syscall.Stat_t)

		var remap bool

		uid := stat.Uid
		if uid == 0 {
			remap = true
			uid = toUid
		}

		gid := stat.Gid
		if gid == 0 {
			remap = true
			gid = toGid
		}

		if !remap {
			return nil
		}

		// be sure the lchown the path as to not de-reference the symlink to a host file
		return os.Lchown(path, int(uid), int(gid))
	}
}

func importImage(ctx context.Context, client *containerd.Client, path string) (containerd.Image, error) {
	imageFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer imageFile.Close()

	logrus.Info("importing")

	images, err := client.Import(ctx, imageFile, containerd.WithIndexName("some-ref"))
	if err != nil {
		return nil, err
	}

	var image containerd.Image
	for _, i := range images {
		image = containerd.NewImage(client, i)

		err = image.Unpack(ctx, containerd.DefaultSnapshotter)
		if err != nil {
			return nil, err
		}
	}

	logrus.Debug("image ready")

	if image == nil {
		return nil, fmt.Errorf("no image found in archive: %s", path)
	}

	return image, nil
}
