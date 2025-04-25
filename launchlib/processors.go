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
	cpuWeightName = "cpu.weight"
	// DefaultProcessorCount is the default number of processors (2).
	DefaultProcessorCount = 2
	// WeightPerProcessor is the CPU weight per processor in cgroup v2 (100).
	WeightPerProcessor = 100
)

type ProcessorCounter interface {
	ProcessorCount() (int, error)
}

var defaultFS = os.DirFS("/")

var DefaultProcessorCounter = NewCGroupProcessorCounter(defaultFS)

type CGroupProcessorCounter struct {
	cgroupPaths CGroupPather
	fs          fs.FS
}

func NewCGroupProcessorCounter(filesystem fs.FS) ProcessorCounter {
	return CGroupProcessorCounter{cgroupPaths: NewCGroupV2Pather(filesystem), fs: filesystem}
}

func (c CGroupProcessorCounter) ProcessorCount() (int, error) {
	cpuCgroupPath, err := c.cgroupPaths.Path(cpuGroupName)
	if err != nil {
		return DefaultProcessorCount, errors.Wrap(err, "failed to get path to cpu cgroup")
	}

	cpuWeightFilepath := filepath.Join(cpuCgroupPath, cpuWeightName)
	cpuWeightFile, err := c.fs.Open(convertToFSPath(cpuWeightFilepath))
	if err != nil {
		return DefaultProcessorCount, errors.Wrapf(err, "unable to open cpu.weight at expected location: %s", cpuWeightFilepath)
	}
	var closeErr error
	defer func() {
		if cerr := cpuWeightFile.Close(); cerr != nil && err == nil {
			closeErr = errors.Wrap(cerr, "failed to close cpu.weight")
		}
	}()

	cpuWeightBytes, err := io.ReadAll(cpuWeightFile)
	if err != nil {
		return DefaultProcessorCount, errors.Wrapf(err, "unable to read cpu.weight")
	}
	cpuWeight, err := strconv.Atoi(strings.TrimSpace(string(cpuWeightBytes)))
	if err != nil {
		return DefaultProcessorCount, errors.New("unable to convert cpu.weight value to expected type")
	}

	if closeErr != nil {
		return DefaultProcessorCount, closeErr
	}

	// Convert weight to processor count (weight of 100 = 1 processor)
	processors := cpuWeight / WeightPerProcessor
	if processors < 1 {
		processors = 1
	}

	virtualCPUs := runtime.NumCPU()
	cpuShareCPUs := math.Floor(float64(processors / 100))

	// We think we will be better off providing >1 cores in cases where the underlying host has multiple CPUs to ensure
	// smaller applications don't get blocked by too few GC threads, as well as issues in many concurrent data-structures
	// which assume they must operate differently when ActiveProcessorCount=1 because parallel computation is impossible.
	// https://github.com/palantir/go-java-launcher/issues/313
	if virtualCPUs == 1 {
		return 1, nil
	}
	return int(math.Max(2.0, math.Min(cpuShareCPUs, float64(virtualCPUs)))), nil
}
