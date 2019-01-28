package houdini

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/garden"
)

func (container *container) setup() error {
	if container.hasRootfs {
		for _, dir := range []string{"/proc", "/dev", "/sys"} {
			dest := filepath.Join(container.workDir, dir)

			err := os.MkdirAll(dest, 0755)
			if err != nil {
				return fmt.Errorf("failed to create target for bind mount: %s", err)
			}

			err = syscall.Mount(dir, dest, "none", syscall.MS_BIND|syscall.MS_RDONLY, "")
			if err != nil {
				return err
			}
		}

		for _, file := range []string{"/etc/resolv.conf", "/etc/hosts"} {
			dest := filepath.Join(container.workDir, file)

			f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to create target for bind mount: %s", err)
			}

			err = f.Close()
			if err != nil {
				return err
			}

			err = syscall.Mount(file, dest, "none", syscall.MS_BIND|syscall.MS_RDONLY, "")
			if err != nil {
				return err
			}
		}
	}

	for _, bm := range container.spec.BindMounts {
		dest := filepath.Join(container.workDir, bm.DstPath)

		err := os.MkdirAll(dest, 0755)
		if err != nil {
			return fmt.Errorf("failed to create target for bind mount: %s", err)
		}

		flags := uintptr(syscall.MS_BIND)
		if bm.Mode == garden.BindMountModeRO {
			flags |= syscall.MS_RDONLY
		}

		err = syscall.Mount(bm.SrcPath, dest, "none", flags, "")
		if err != nil {
			return err
		}
	}

	return nil
}

const defaultRootPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
// const defaultPath = "/usr/local/bin:/usr/bin:/bin"

func (container *container) path() string {
	var path string
	for _, env := range container.env {
		segs := strings.SplitN(env, "=", 2)
		if len(segs) < 2 {
			continue
		}

		if segs[0] == "PATH" {
			path = segs[1]
		}
	}

	if !container.hasRootfs {
		if path == "" {
			path = os.Getenv("PATH")
		}

		return path
	}

	if path == "" {
		// assume running as root for now, since Houdini doesn't currently support
		// running as a user
		path = defaultRootPath
	}

	var scopedPath string
	for _, dir := range filepath.SplitList(path) {
		if scopedPath != "" {
			scopedPath += string(filepath.ListSeparator)
		}

		scopedPath += container.workDir + dir
	}

	return scopedPath
}

func (container *container) cmd(spec garden.ProcessSpec) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	if container.hasRootfs {
		path := spec.Path

		if !strings.Contains(path, "/") {
			// find executable within container's $PATH

			absPath, err := lookPath(path, container.path())
			if err != nil {
				return nil, garden.ExecutableNotFoundError{
					Message: err.Error(),
				}
			}

			// correct path so that it's absolute from the rootfs
			path = strings.TrimPrefix(absPath, container.workDir)
		}

		cmd = exec.Command(path, spec.Args...)

		if spec.Dir != "" {
			cmd.Dir = spec.Dir
		} else {
			cmd.Dir = "/"
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Chroot: container.workDir,
		}
	} else {
		cmd = exec.Command(spec.Path, spec.Args...)
		cmd.Dir = filepath.Join(container.workDir, spec.Dir)
	}

	cmd.Env = append(os.Environ(), append(container.env, spec.Env...)...)

	return cmd, nil
}

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return os.ErrPermission
}

// based on exec.LookPath from stdlib
func lookPath(file string, path string) (string, error) {
	if strings.Contains(file, "/") {
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", &exec.Error{Name: file, Err: err}
	}

	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}

	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}
