package houdini

import (
	"bufio"
	"fmt"
	"os"
)

type idMap string

const defaultUIDMap idMap = "/proc/self/uid_map"
const defaultGIDMap idMap = "/proc/self/gid_map"

const maxInt = uint32(^uint32(0) >> 1)

func (u idMap) MaxValid() (uint32, error) {
	f, err := os.Open(string(u))
	if err != nil {
		return 0, err
	}

	var m uint32
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var container, host, size uint32
		if _, err := fmt.Sscanf(scanner.Text(), "%d %d %d", &container, &host, &size); err != nil {
			return 0, err
		}

		m = minUint32(maxUint32(m, container+size-1), maxInt)
	}

	return m, nil
}

func maxUint32(a, b uint32) uint32 {
	if a > b {
		return a
	}

	return b
}

func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}

	return b
}

