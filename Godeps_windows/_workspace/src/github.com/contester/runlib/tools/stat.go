package tools

import (
	"github.com/contester/runlib/contester_proto"
	"os"
	"code.google.com/p/goprotobuf/proto"
)

func StatFile(name string, hash_it bool) (*contester_proto.FileStat, error) {
	result := &contester_proto.FileStat{}
	result.Name = &name
	info, err := os.Stat(name)
	if err != nil {
		// Handle ERROR_FILE_NOT_FOUND - return no error and nil instead of stat struct
		if IsStatErrorFileNotFound(err) {
			return nil, nil
		}

		return nil, NewError(err, "statFile", "os.Stat")
	}
	if info.IsDir() {
		result.IsDirectory = proto.Bool(true)
	} else {
		result.Size = proto.Uint64(uint64(info.Size()))
		if hash_it {
			checksum, err := HashFileString(name)
			if err != nil {
				return nil, NewError(err, "statFile", "hashFile")
			}
			result.Checksum = &checksum
		}
	}
	return result, nil
}
