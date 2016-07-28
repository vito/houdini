// +build !windows

package houdini

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
)

func RunCommandAsUser(cmd *exec.Cmd, spec garden.ProcessSpec) error {
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

	os.Chown(cmd.Dir, int(uid), int(gid))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}

	return nil
}
