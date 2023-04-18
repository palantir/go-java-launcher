package launchlib

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	memGroupName = "memory"
	memLimitName = "memory.limit_in_bytes"
)

type MemoryLimit interface {
	MemoryLimit() (uint64, error)
}

var DefaultMemoryLimit = NewCGroupMemoryLimit(os.DirFS("/"))

type CGroupMemoryLimit struct {
	pather CGroupPather
	fs     fs.FS
}

func NewCGroupMemoryLimit(filesystem fs.FS) MemoryLimit {
	return CGroupMemoryLimit{
		pather: NewCGroupV1Pather(filesystem),
		fs:     filesystem,
	}
}

func (c CGroupMemoryLimit) MemoryLimit() (uint64, error) {
	memoryCGroupPath, err := c.pather.Path(memGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get memory cgroup path")
	}

	memLimitFilepath := filepath.Join(memoryCGroupPath, memLimitName)
	memLimitFile, err := c.fs.Open(convertToFSPath(memLimitFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open memory.limit_in_bytes at expected location: %s", memLimitFilepath)
	}
	memLimitBytes, err := io.ReadAll(memLimitFile)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to read memory.limit_in_bytes")
	}
	memLimit, err := strconv.Atoi(strings.TrimSpace(string(memLimitBytes)))
	if err != nil {
		return 0, errors.New("unable to convert memory.limit_in_bytes value to expected type")
	}
	return uint64(memLimit), nil
}
