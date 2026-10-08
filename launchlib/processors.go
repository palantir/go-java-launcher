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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	cpuGroupName  = CGroupName("cpu")
	cpuSharesName = "cpu.shares"
	cpuWeightName = "cpu.weight"
)

type ProcessorCounter interface {
	ProcessorCount() (uint, error)
}

var defaultFS = os.DirFS("/")

var DefaultCGroupV1ProcessorCounter = NewCGroupV1ProcessorCounter(defaultFS)

type CGroupProcessorCounter struct {
	cgroupPaths CGroupPather
	fs          fs.FS
	isCGroupV2  bool
}

func NewCGroupProcessorCounter(filesystem fs.FS) (ProcessorCounter, error) {
	isCGroupV2, err := IsCGroupV2(filesystem)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine cgroup version")
	}
	if isCGroupV2 {
		return CGroupProcessorCounter{
			cgroupPaths: NewCGroupV2Pather(),
			fs:          filesystem,
			isCGroupV2:  true,
		}, nil
	}
	return NewCGroupV1ProcessorCounter(filesystem), nil
}

func NewCGroupV1ProcessorCounter(filesystem fs.FS) ProcessorCounter {
	return CGroupProcessorCounter{cgroupPaths: NewCGroupV1Pather(filesystem), fs: filesystem}
}

func (c CGroupProcessorCounter) ProcessorCount() (uint, error) {
	cpuCgroupPath, err := c.cgroupPaths.Path(cpuGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get path to cpu cgroup")
	}

	cpuRequestName := cpuSharesName
	if c.isCGroupV2 {
		cpuRequestName = cpuWeightName
	}
	cpuRequestFilepath := filepath.Join(cpuCgroupPath, cpuRequestName)
	cpuRequestFile, err := c.fs.Open(convertToFSPath(cpuRequestFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open %s at expected location: %s", cpuRequestName, cpuRequestFilepath)
	}
	defer func() {
		_ = cpuRequestFile.Close()
	}()
	cpuRequestBytes, err := io.ReadAll(cpuRequestFile)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to read %s", cpuRequestName)
	}
	cpuShares, err := strconv.Atoi(strings.TrimSpace(string(cpuRequestBytes)))
	if err != nil {
		return 0, errors.Errorf("unable to convert %s value to expected type", cpuRequestName)
	}
	if cpuShares <= 0 || (c.isCGroupV2 && cpuShares > 10000) {
		return 0, errors.Errorf("invalid %s value: %d", cpuRequestName, cpuShares)
	}
	if c.isCGroupV2 {
		// Reverse the container runtime conversion from shares [2, 262144] to weight [1, 10000].
		cpuShares = 2 + (cpuShares-1)*262142/9999
	}
	return uint(cpuShares / 1024), nil
}
