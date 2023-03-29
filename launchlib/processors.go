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

package launchlib

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	selfCGroup    = "/proc/self/cgroup"
	selfMountinfo = "/proc/self/mountinfo"
	cpuGroupName  = "cpu"
	cpuSharesName = "cpu.shares"
)

type ProcessorCounter interface {
	ProcessorCount() (uint, error)
}

var DefaultCGroupV1ProcessorCounter = CGroupV1ProcessorCounter{
	fs: os.DirFS("/"),
}

type CGroupV1ProcessorCounter struct {
	fs fs.FS
}

func NewCGroupV1ProcessorCounter(filesystem fs.FS) ProcessorCounter {
	return CGroupV1ProcessorCounter{fs: filesystem}
}

func (c CGroupV1ProcessorCounter) ProcessorCount() (uint, error) {
	cpuCgroupPath, err := c.cpuCGroupPath()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get path to cpu cgroup")
	}

	cpuSharesFilepath := filepath.Join(cpuCgroupPath, cpuSharesName)
	cpuSharesFile, err := c.fs.Open(c.convertToFSPath(cpuSharesFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open cpu.shares at expected location: %s", cpuSharesFilepath)
	}
	cpuShareBytes, err := io.ReadAll(cpuSharesFile)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to read cpu.shares")
	}
	cpuShares, err := strconv.Atoi(string(cpuShareBytes))
	if err != nil {
		return 0, errors.New("unable to convert cpu.shares value to expected type")
	}

	virtualCPUs := runtime.NumCPU()
	cpuShareCPUs := math.Floor(float64(cpuShares / 1024))

	// We think we will be better off providing >1 cores in cases where the underlying host has multiple CPUs to ensure
	// smaller applications don't get blocked by too few GC threads, as well as issues in many concurrent data-structures
	// which assume they must operate differently when ActiveProcessorCount=1 because parallel computation is impossible.
	// https://github.com/palantir/go-java-launcher/issues/313
	if virtualCPUs == 1 {
		return 1, nil
	}
	return uint(math.Max(2.0, math.Min(cpuShareCPUs, float64(virtualCPUs)))), nil
}

func (c CGroupV1ProcessorCounter) cpuCGroupPath() (string, error) {
	selfCGroupFile, err := c.fs.Open(c.convertToFSPath(selfCGroup))
	if err != nil {
		return "", errors.Wrap(err, "failed to open cgroup file")
	}
	cpuCGroupRootMountPath, err := c.getCPUCGroupPath(selfCGroupFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to get cpu cgroup information from cgroup entries")
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

		if !bytes.Equal(rootMount, []byte(cpuCGroupRootMountPath)) {
			continue
		}
		// options and mount points may contain multiple cgroup types within them, separated by commas (e.g. cpu,cpuacct)
		for _, option := range bytes.Split(options, []byte(",")) {
			if bytes.Equal(option, []byte(cpuGroupName)) {
				mountBases := strings.Split(filepath.Base(string(mount)), ",")
				if len(mountBases) == 1 {
					return string(mount), nil
				}
				for _, mountBase := range mountBases {
					if mountBase == cpuGroupName {
						return filepath.Join(filepath.Dir(string(mount)), mountBase), nil
					}
				}
			}
		}
	}
	return "", errors.New("unable to find cpu cgroup mount path")
}

func (c CGroupV1ProcessorCounter) getCPUCGroupPath(r io.Reader) (string, error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		cgroupParts := strings.Split(s.Text(), ":")
		if len(cgroupParts) < 3 {
			continue
		}
		cgroupNames := cgroupParts[1]
		for _, subgroup := range strings.Split(cgroupNames, ",") {
			if subgroup == cpuGroupName {
				return cgroupParts[2], nil
			}
		}
	}
	return "", errors.New("unable to find cpu cgroup mount path in cgroup entries")
}

func (c CGroupV1ProcessorCounter) convertToFSPath(path string) string {
	// The io.fs package has some path quirks, the biggest being that it expects to work with unrooted paths, and will
	// reject any paths with leading slashes as invalid. To deal with this, we have to remove any trailing slashes that
	// we get back from parsing any
	// https://pkg.go.dev/io/fs#ValidPath
	return strings.TrimPrefix(path, "/")
}
