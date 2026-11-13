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

const (
	// cgroup v1 cpu.shares per core, as written by Kubernetes and container runtimes.
	cpuSharesPerCore = 1024
	maxCPUShares     = 262144
	maxCPUWeight     = 10000
)

// ProcessorCount returns the cgroup CPU request rounded down to whole cores, and at least 1.
func (c CGroupProcessorCounter) ProcessorCount() (uint, error) {
	cores, err := c.cpuRequestCores()
	if err != nil {
		return 0, err
	}
	return uint(max(1, math.Floor(cores))), nil
}

// cpuRequestCores returns the container CPU request in cores, read from cpu.shares on cgroup v1 or cpu.weight on
// cgroup v2. The value is not rounded, so callers can round, floor or ceil it as needed. Kubernetes writes
// millicores * 1024 / 1000 shares (at least 2), so a request with no CPU set reads as 2/1024.
//
// On cgroup v2 the shares are recovered from the weight, which is lossy since each weight covers a range of shares.
// The returned value is the top of that range, so it can be slightly above the actual request (e.g. a 200m request
// reads as about 0.202 cores). Callers comparing against a threshold should account for this.
func (c CGroupProcessorCounter) cpuRequestCores() (float64, error) {
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
	value, err := strconv.Atoi(strings.TrimSpace(string(cpuRequestBytes)))
	if err != nil {
		return 0, errors.Errorf("unable to convert %s value to expected type", cpuRequestName)
	}
	if value <= 0 || (c.isCGroupV2 && value > maxCPUWeight) {
		return 0, errors.Errorf("invalid %s value: %d", cpuRequestName, value)
	}
	if !c.isCGroupV2 {
		return float64(value) / cpuSharesPerCore, nil
	}
	return cpuWeightToShares(value) / cpuSharesPerCore, nil
}

// cpuWeightToShares inverts the shares to weight conversion used by runc >= 1.3.2 and crun >= 1.23:
// weight = ceil(10^((l*l+125*l)/612 - 7/34)), where l = log2(shares).
// See https://github.com/opencontainers/cgroups/blob/v0.0.4/utils.go.
//
// Inverting at the weight itself gives the top of the range of shares that map to it, so runtime rounding never loses
// part of the request. Older runtimes used a linear mapping that produces a lower weight for the same shares, which
// reads back as a smaller request. There's no way to tell which runtime wrote the weight, so we assume the current one.
func cpuWeightToShares(weight int) float64 {
	// Clamp in case floating point error pushes the maximum weight past the shares maximum.
	return min(maxCPUShares, math.Exp2((math.Sqrt(16129+2448*math.Log10(float64(weight)))-125)/2))
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
