package launchlib

import (
	"bufio"
	"bytes"
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

type CGroupPather interface {
	GetPath(module string) (string, error)
}

var DefaultCGroupV1Pather = CGroupV1Pather{
	fs: os.DirFS("/"),
}

type CGroupV1Pather struct {
	fs fs.FS
}

func NewCGroupV1Pather(filesystem fs.FS) CGroupV1Pather {
	return CGroupV1Pather{fs: filesystem}
}

func (c CGroupV1Pather) GetPath(module string) (string, error) {
	selfCGroupFile, err := c.fs.Open(c.convertToFSPath(selfCGroup))
	if err != nil {
		return "", errors.Wrap(err, "failed to open cgroup file")
	}
	cCGroupRootMountPath, err := c.getCGroupPath(selfCGroupFile, module)
	if err != nil {
		return "", errors.Wrap(err, "failed to get cgroup information from cgroup entries")
	}

	selfMountinfoFile, err := c.fs.Open(c.convertToFSPath(selfMountinfo))
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

		if !bytes.Equal(rootMount, []byte(cCGroupRootMountPath)) {
			continue
		}
		// options and mount points may contain multiple cgroup types within them, separated by commas (e.g. cpu,cpuacct)
		for _, option := range bytes.Split(options, []byte(",")) {
			if bytes.Equal(option, []byte(module)) {
				mountBases := strings.Split(filepath.Base(string(mount)), ",")
				if len(mountBases) == 1 {
					return string(mount), nil
				}
				for _, mountBase := range mountBases {
					if mountBase == module {
						return filepath.Join(filepath.Dir(string(mount)), mountBase), nil
					}
				}
			}
		}
	}
	return "", errors.Errorf("unable to find mount path for cgroup module %s", module)
}

func (c CGroupV1Pather) getCGroupPath(r io.Reader, module string) (string, error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		cgroupParts := strings.Split(s.Text(), ":")
		if len(cgroupParts) < 3 {
			continue
		}
		cgroupNames := cgroupParts[1]
		for _, subgroup := range strings.Split(cgroupNames, ",") {
			if subgroup == module {
				return cgroupParts[2], nil
			}
		}
	}
	return "", errors.New("unable to find cgroup mount path in cgroup entries")
}
