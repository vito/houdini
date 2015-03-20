// +build linux darwin

package tools

import "syscall"

// Linux-specific "file not found" error aka ENOENT
func IsFileNotFoundError(err error) bool {
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == syscall.ENOENT {
			return true
		}
	}
	return false
}
