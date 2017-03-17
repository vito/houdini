// +build !linux

package houdini

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
)

func (container *container) setup() error {
	for _, bm := range container.spec.BindMounts {
		if bm.Mode == garden.BindMountModeRO {
			return errors.New("read-only bind mounts are unsupported")
		}

		dest := filepath.Join(container.workDir, bm.DstPath)
		_, err := os.Stat(dest)
		if err == nil {
			err = os.Remove(dest)
			if err != nil {
				return fmt.Errorf("failed to remove destination for bind mount: %s", err)
			}
		}

		err = os.MkdirAll(filepath.Dir(dest), 0755)
		if err != nil {
			return fmt.Errorf("failed to create parent dir for bind mount: %s", err)
		}

		absSrc, err := filepath.Abs(bm.SrcPath)
		if err != nil {
			return fmt.Errorf("failed to resolve source path: %s", err)
		}

		// windows symlinks ("junctions") support directories, but not hard-links
		// darwin hardlinks have strange restrictions
		// symlinks behave reasonably similar to bind mounts on OS X (unlike Linux)
		err = os.Symlink(absSrc, dest)
		if err != nil {
			return fmt.Errorf("failed to create hardlink for bind mount: %s", err)
		}
	}

	return nil
}
