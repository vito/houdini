package houdini

import (
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
)

func RunCommandAsUser(cmd *exec.Cmd, spec garden.ProcessSpec) error {
	// cmd.SysProcAttr for windows doesn't have a credentials struct object like unix
	return nil
}
