package tools

import "syscall"

// Windows-specific "file not found", aka ERROR_FILE_NOT_FOUND
func IsFileNotFoundError(err error) bool {
	if err != nil {
		if err == syscall.ERROR_FILE_NOT_FOUND {
			return true
		}
	}
	return false
}
