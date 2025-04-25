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

type CGroupName string

type CGroupPather interface {
	Path(name CGroupName) (string, error)
}

var DefaultCGroupV1Pather = CGroupV1Pather{
	fs: os.DirFS("/"),
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

func convertToFSPath(path string) string {
	// The io.fs package has some path quirks, the biggest being that it expects to work with unrooted paths, and will
	// reject any paths with leading slashes as invalid. To deal with this, we have to remove any trailing slashes that
	// we get back from parsing any
	// https://pkg.go.dev/io/fs#ValidPath
	return strings.TrimPrefix(path, "/")
}

type CGroupV2Pather struct {
	fs fs.FS
}

func NewCGroupV2Pather(filesystem fs.FS) CGroupPather {
	return CGroupV2Pather{fs: filesystem}
}

// Path implements CGroupPather
func (c CGroupV2Pather) Path(name CGroupName) (string, error) {
	// Check if cgroup v2 unified hierarchy is mounted
	mountinfo, err := c.fs.Open(convertToFSPath(selfMountinfo))
	if err != nil {
		return "", errors.Wrap(err, "failed to open mountinfo file")
	}
	var closeErr error
	defer func() {
		if cerr := mountinfo.Close(); cerr != nil && err == nil {
			closeErr = errors.Wrap(cerr, "failed to close mountinfo file")
		}
	}()

	// Read and parse mountinfo to find cgroup2 mount point
	content, err := io.ReadAll(mountinfo)
	if err != nil {
		return "", errors.Wrap(err, "failed to read mountinfo")
	}

	var cgroupv2Mount string
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}

		// Parse mountinfo line according to its format:
		// 36 35 0:31 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:12 - cgroup2 cgroup2 rw,nsdelegate,memory_recursiveprot
		// Format: ID ParentID Major:Minor Root Mountpoint MountOptions - FSType Source FSOptions
		parts := bytes.Split(line, []byte(" - "))
		if len(parts) != 2 {
			continue
		}

		// Get filesystem type from the second part
		fsInfo := bytes.Fields(parts[1])
		if len(fsInfo) < 1 || !bytes.Equal(fsInfo[0], []byte("cgroup2")) {
			continue
		}

		// Get mount point from the first part
		mountFields := bytes.Fields(parts[0])
		if len(mountFields) < 5 {
			continue
		}
		cgroupv2Mount = string(mountFields[4])
		break
	}

	if cgroupv2Mount == "" {
		return "", errors.New("cgroup v2 unified hierarchy not mounted")
	}

	// Check if the requested controller is enabled
	controllersFile := filepath.Join(cgroupv2Mount, "cgroup.controllers")
	controllers, err := c.fs.Open(convertToFSPath(controllersFile))
	if err != nil {
		return "", errors.Wrap(err, "failed to read available controllers")
	}
	defer func() {
		if cerr := controllers.Close(); cerr != nil && err == nil {
			closeErr = errors.Wrap(cerr, "failed to close controllers file")
		}
	}()

	content, err = io.ReadAll(controllers)
	if err != nil {
		return "", errors.Wrap(err, "failed to read controllers file")
	}

	found := false
	for _, controller := range bytes.Fields(content) {
		if bytes.Equal(controller, []byte(name)) {
			found = true
			break
		}
	}

	if !found {
		return "", errors.Errorf("controller %q not enabled in cgroup v2 hierarchy", name)
	}

	if closeErr != nil {
		return "", closeErr
	}

	// In cgroup v2, all controllers are mounted at the unified hierarchy root
	return cgroupv2Mount, nil
}
