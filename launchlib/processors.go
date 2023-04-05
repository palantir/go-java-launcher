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
	cpuGroupName  = CGroupName("cpu")
	cpuSharesName = "cpu.shares"
)

type ProcessorCounter interface {
	ProcessorCount() (uint, error)
}

var defaultFS = os.DirFS("/")

var DefaultCGroupV1ProcessorCounter = CGroupV1ProcessorCounter{
	cgroupPaths: NewCGroupV1Pather(defaultFS),
	fs:          defaultFS,
}

type CGroupV1ProcessorCounter struct {
	cgroupPaths CGroupPather
	fs          fs.FS
}

func NewCGroupV1ProcessorCounter(filesystem fs.FS) ProcessorCounter {
	return CGroupV1ProcessorCounter{cgroupPaths: NewCGroupV1Pather(filesystem), fs: filesystem}
}

func (c CGroupV1ProcessorCounter) ProcessorCount() (uint, error) {
	cpuCgroupPath, err := c.cgroupPaths.Path(cpuGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get path to cpu cgroup")
	}

	cpuSharesFilepath := filepath.Join(cpuCgroupPath, cpuSharesName)
	cpuSharesFile, err := c.fs.Open(convertToFSPath(cpuSharesFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open cpu.shares at expected location: %s", cpuSharesFilepath)
	}
	cpuShareBytes, err := io.ReadAll(cpuSharesFile)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to read cpu.shares")
	}
	cpuShares, err := strconv.Atoi(strings.TrimSpace(string(cpuShareBytes)))
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
