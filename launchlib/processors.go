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
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	cpuGroupName  = CGroupName("cpu")
	cpuSharesName = "cpu.shares"
	cpuWeightName = "cpu.weight"
	cpuMaxName    = "cpu.max"
	cpuQuotaName  = "cpu.cfs_quota_us"
	cpuPeriodName = "cpu.cfs_period_us"
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
	counter, err := newCGroupProcessorCounter(filesystem)
	if err != nil {
		return nil, err
	}
	return counter, nil
}

func newCGroupProcessorCounter(filesystem fs.FS) (CGroupProcessorCounter, error) {
	isCGroupV2, err := IsCGroupV2(filesystem)
	if err != nil {
		return CGroupProcessorCounter{}, errors.Wrap(err, "failed to determine cgroup version")
	}
	cgroupPaths := NewCGroupV1Pather(filesystem)
	if isCGroupV2 {
		cgroupPaths = NewCGroupV2Pather()
	}
	return CGroupProcessorCounter{cgroupPaths: cgroupPaths, fs: filesystem, isCGroupV2: isCGroupV2}, nil
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
		// runc >= 1.3.2 and crun >= 1.23 use weight = ceil(10^((l*l+125*l)/612 - 7/34)),
		// where l = log2(shares). See https://github.com/opencontainers/cgroups/blob/v0.0.4/utils.go.
		// Invert at the weight bucket's upper bound so runtime rounding does not lose whole cores.
		// Clamp to the shares maximum before truncating to cores.
		cpuShares = int(min(262144, math.Exp2((math.Sqrt(16129+2448*math.Log10(float64(cpuShares)))-125)/2)))
	}
	return uint(max(1, cpuShares/1024)), nil
}

// clampToCPUQuota matches the JVM's quota-based processor count, without exceeding CPU affinity.
func (c CGroupProcessorCounter) clampToCPUQuota(processors int) (int, error) {
	cpuCgroupPath, err := c.cgroupPaths.Path(cpuGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get path to cpu cgroup")
	}
	readValue := func(name string) (string, error) {
		content, err := fs.ReadFile(c.fs, convertToFSPath(filepath.Join(cpuCgroupPath, name)))
		if err != nil {
			return "", errors.Wrapf(err, "unable to read %s", name)
		}
		return strings.TrimSpace(string(content)), nil
	}
	var quota, period string
	if c.isCGroupV2 {
		value, err := readValue(cpuMaxName)
		if err != nil {
			return 0, err
		}
		fields := strings.Fields(value)
		if len(fields) != 2 {
			return 0, errors.Errorf("invalid %s value: %q", cpuMaxName, value)
		}
		quota, period = fields[0], fields[1]
	} else {
		quota, err = readValue(cpuQuotaName)
		if err != nil {
			return 0, err
		}
		period, err = readValue(cpuPeriodName)
		if err != nil {
			return 0, err
		}
	}
	periodMicros, err := strconv.ParseInt(period, 10, 64)
	if err != nil || periodMicros <= 0 {
		return 0, errors.Errorf("invalid CPU period: %q", period)
	}
	if (c.isCGroupV2 && quota == "max") || (!c.isCGroupV2 && quota == "-1") {
		return processors, nil
	}
	quotaMicros, err := strconv.ParseInt(quota, 10, 64)
	if err != nil || quotaMicros <= 0 {
		return 0, errors.Errorf("invalid CPU quota: %q", quota)
	}
	// Integer ceil(quota/period), avoiding overflow when adding period to quota.
	quotaProcessors := 1 + (quotaMicros-1)/periodMicros
	return int(min(int64(processors), quotaProcessors)), nil
}
