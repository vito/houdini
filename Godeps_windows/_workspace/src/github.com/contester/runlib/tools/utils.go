package tools

import (
	"unsafe"
	"os"
)

// Return byte slice of given size, aligned at given offset.
func AlignedBuffer(size, offset int) []byte {
	buf := make([]byte, size+offset)
	ofs := int((uintptr(offset) - (uintptr(unsafe.Pointer(&buf[0])) % uintptr(offset))) % uintptr(offset))
	return buf[ofs : ofs+size]
}

// Return true if the error provided (usually from os.Stat) is a "file not found" error.
func IsStatErrorFileNotFound(err error) bool {
	if err != nil {
		if path_err, ok := err.(*os.PathError); ok && IsFileNotFoundError(path_err.Err) {
			return true
		}
	}
	return false
}
