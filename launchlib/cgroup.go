// Copyright 2023 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package launchlib provides functionality for launching Java processes with cgroup v2 resource limits.
package launchlib

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	selfMountinfo = "/proc/self/mountinfo"
)

// CGroupName represents a cgroup v2 controller name (e.g. "cpu", "memory").
type CGroupName string

// CGroupPather provides functionality to determine the path to a cgroup v2 controller.
type CGroupPather interface {
	// Path returns the path to the cgroup v2 controller with the given name.
	// Returns an error if the controller is not available or enabled.
	Path(name CGroupName) (string, error)
}

// DefaultCGroupPather is the default CGroupPather that uses the system's filesystem.
var DefaultCGroupPather = NewCGroupV2Pather(os.DirFS("/"))

// CGroupV2Pather implements CGroupPather for cgroup v2 unified hierarchy.
type CGroupV2Pather struct {
	fs fs.FS
}

// NewCGroupV2Pather creates a new CGroupV2Pather that uses the given filesystem.
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

func convertToFSPath(path string) string {
	// The io.fs package has some path quirks, the biggest being that it expects to work with unrooted paths, and will
	// reject any paths with leading slashes as invalid. To deal with this, we have to remove any trailing slashes that
	// we get back from parsing any
	// https://pkg.go.dev/io/fs#ValidPath
	return strings.TrimPrefix(path, "/")
}
