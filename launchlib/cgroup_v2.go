package launchlib

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	cgroupV2MountPoint = "/sys/fs/cgroup"
	cgroupV2MountType  = "cgroup2"
	memLimitV2Name     = "memory.max"
)

// IsCGroupV2 checks if the system is using cgroups v2
func IsCGroupV2(filesystem fs.FS) bool {
	mountinfoFile, err := filesystem.Open(convertToFSPath(selfMountinfo))
	if err != nil {
		return false
	}
	defer func() {
		_ = mountinfoFile.Close()
	}()

	scanner := bufio.NewScanner(mountinfoFile)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		if fields[len(fields)-1] == cgroupV2MountType && fields[4] == cgroupV2MountPoint {
			return true
		}
	}
	return false
}

var DefaultCGroupV2Pather = CGroupV2Pather{
	fs: os.DirFS("/"),
}

type CGroupV2Pather struct {
	fs fs.FS
}

func NewCGroupV2Pather(filesystem fs.FS) CGroupPather {
	return CGroupV2Pather{fs: filesystem}
}

// Path implements CGroupPather for cgroups v2
func (c CGroupV2Pather) Path(name CGroupName) (string, error) {
	selfCGroupFile, err := c.fs.Open(convertToFSPath(selfCGroup))
	if err != nil {
		return "", errors.Wrap(err, "failed to open cgroup file")
	}
	defer func() {
		_ = selfCGroupFile.Close()
	}()

	scanner := bufio.NewScanner(selfCGroupFile)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 3 {
			continue
		}
		// In cgroups v2, there's only one hierarchy, so we just need the path
		return filepath.Join(cgroupV2MountPoint, parts[2]), nil
	}
	return "", errors.New("failed to find cgroup path in cgroups v2")
}

type CGroupV2MemoryLimit struct {
	pather CGroupPather
	fs     fs.FS
}

func NewCGroupV2MemoryLimit(filesystem fs.FS) MemoryLimit {
	return CGroupV2MemoryLimit{
		pather: NewCGroupV2Pather(filesystem),
		fs:     filesystem,
	}
}

func (c CGroupV2MemoryLimit) MemoryLimitInBytes() (uint64, error) {
	cgroupPath, err := c.pather.Path(memGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get cgroup path")
	}

	memLimitFilepath := filepath.Join(cgroupPath, memLimitV2Name)
	memLimitFile, err := c.fs.Open(convertToFSPath(memLimitFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open memory.max at expected location: %s", memLimitFilepath)
	}
	defer func() {
		_ = memLimitFile.Close()
	}()

	memLimitBytes, err := io.ReadAll(memLimitFile)
	if err != nil {
		return 0, errors.Wrap(err, "unable to read memory.max")
	}

	// In cgroups v2, "max" means unlimited
	memLimitStr := strings.TrimSpace(string(memLimitBytes))
	if memLimitStr == "max" {
		return 0, errors.New("no memory limit set (unlimited)")
	}

	memLimit, err := strconv.ParseUint(memLimitStr, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "unable to convert memory.max value to expected type")
	}
	return memLimit, nil
}
