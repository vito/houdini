// +build !windows

package houdini

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
)

func setUser(cmd *exec.Cmd, spec garden.ProcessSpec) error {
	runAs, err := user.Lookup(spec.User)
	if err != nil {
		return err
	}
	uid, err := strconv.ParseUint(runAs.Uid, 10, 32)
	if err != nil {
		return err
	}
	gid, err := strconv.ParseUint(runAs.Gid, 10, 32)
	if err != nil {
		return err
	}

	if err := chownR(cmd.Dir, int(uid), int(gid)); err != nil {
		return err
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	return nil
}

func chownR(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = os.Chown(name, uid, gid)
		}
		return err
	})
}
