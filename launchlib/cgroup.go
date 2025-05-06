package launchlib

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	selfCGroup    = "/proc/self/cgroup"
	selfMountinfo = "/proc/self/mountinfo"
)

type CGroupName string

type CGroupPather interface {
	Path(name CGroupName) (string, error)
}

var DefaultCGroupV1Pather = CGroupV1Pather{
	fs: os.DirFS("/"),
}

var DefaultCGroupV2Pather = CGroupV2Pather{
	fs: os.DirFS("/"),
}

func IsCGroupV2(filesystem fs.FS) (bool, error) {
	file, err := filesystem.Open(convertToFSPath(selfMountinfo))
	if err != nil {
		return false, fmt.Errorf("failed to open %s: %w", selfMountinfo, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", selfMountinfo, err)
	}

	return bytes.Contains(data, []byte("cgroup2")), nil
}

type CGroupV1Pather struct {
	fs fs.FS
}

func NewCGroupV1Pather(filesystem fs.FS) CGroupPather {
	return CGroupV1Pather{fs: filesystem}
}

// Path implements CGroupPather
func (c CGroupV1Pather) Path(name CGroupName) (string, error) {
	selfCGroupFile, err := c.fs.Open(convertToFSPath(selfCGroup))
	if err != nil {
		return "", errors.Wrap(err, "failed to open cgroup file")
	}
	cgroupModuleRootMountPath, err := c.getCGroupPath(selfCGroupFile, name)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get cgroup information for module %s from cgroup entries", name)
	}

	selfMountinfoFile, err := c.fs.Open(convertToFSPath(selfMountinfo))
	if err != nil {
		return "", errors.Wrap(err, "failed to open mountinfo file")
	}
	mountinfo, err := io.ReadAll(selfMountinfoFile)
	if err != nil {
		return "", err
	}

	// iterate over mount points, filtering to entries which contain the path of our subsystem and the name of our subsystem
	for _, entry := range bytes.Split(mountinfo, []byte("\n")) {
		fields := bytes.Fields(entry)
		if len(fields) < 10 {
			continue
		}

		rootMount, mount, options := fields[3], fields[4], fields[len(fields)-1]

		if !bytes.Equal(rootMount, []byte(cgroupModuleRootMountPath)) {
			continue
		}
		// options and mount points may contain multiple cgroup types within them, separated by commas (e.g. cpu,cpuacct)
		for _, option := range bytes.Split(options, []byte(",")) {
			if bytes.Equal(option, []byte(name)) {
				mountBases := strings.Split(filepath.Base(string(mount)), ",")
				if len(mountBases) == 1 {
					return string(mount), nil
				}
				for _, mountBase := range mountBases {
					if mountBase == string(name) {
						return filepath.Join(filepath.Dir(string(mount)), mountBase), nil
					}
				}
			}
		}
	}
	return "", errors.Errorf("unable to find cgroup mount path for module %s", name)
}

func (c CGroupV1Pather) getCGroupPath(r io.Reader, name CGroupName) (string, error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		cgroupParts := strings.Split(s.Text(), ":")
		if len(cgroupParts) < 3 {
			continue
		}
		cgroupNames := cgroupParts[1]
		for _, subgroup := range strings.Split(cgroupNames, ",") {
			if subgroup == string(name) {
				return cgroupParts[2], nil
			}
		}
	}
	return "", errors.Errorf("unable to find cgroup mount path for module %s in cgroup entries", name)
}

type CGroupV2Pather struct {
	fs fs.FS
}

func NewCGroupV2Pather(filesystem fs.FS) CGroupPather {
	return CGroupV2Pather{fs: filesystem}
}

func (c CGroupV2Pather) Path(name CGroupName) (string, error) {
	// In cgroup v2, all cgroups are mounted under /sys/fs/cgroup
	// The memory cgroup is mounted at /sys/fs/cgroup/memory.max
	return "/sys/fs/cgroup", nil
}

func convertToFSPath(path string) string {
	// The io.fs package has some path quirks, the biggest being that it expects to work with unrooted paths, and will
	// reject any paths with leading slashes as invalid. To deal with this, we have to remove any trailing slashes that
	// we get back from parsing any
	// https://pkg.go.dev/io/fs#ValidPath
	return strings.TrimPrefix(path, "/")
}
