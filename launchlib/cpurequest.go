// Copyright 2026 Palantir Technologies, Inc.
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
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const maxCPUShares = 262144

// cgroupCPURequest is the container CPU request as cgroup v1 cpu.shares or cgroup v2 cpu.weight.
type cgroupCPURequest struct {
	fileName string
	value    uint64
	isWeight bool
}

func readCgroupCPURequest(filesystem fs.FS) (cgroupCPURequest, error) {
	isCGroupV2, err := IsCGroupV2(filesystem)
	if err != nil {
		return cgroupCPURequest{}, errors.Wrap(err, "failed to determine cgroup version")
	}
	pather, fileName := NewCGroupV1Pather(filesystem), cpuSharesName
	if isCGroupV2 {
		pather, fileName = NewCGroupV2Pather(), cpuWeightName
	}
	cpuCGroupPath, err := pather.Path(cpuGroupName)
	if err != nil {
		return cgroupCPURequest{}, errors.Wrap(err, "failed to get cpu cgroup path")
	}
	contents, err := fs.ReadFile(filesystem, convertToFSPath(filepath.Join(cpuCGroupPath, fileName)))
	if err != nil {
		return cgroupCPURequest{}, errors.Wrapf(err, "unable to read %s", fileName)
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil || value == 0 || (isCGroupV2 && value > 10000) {
		return cgroupCPURequest{}, errors.Errorf("invalid %s value: %q", fileName, strings.TrimSpace(string(contents)))
	}
	return cgroupCPURequest{fileName: fileName, value: value, isWeight: isCGroupV2}, nil
}

// above reports whether the request is above millicores.
func (r cgroupCPURequest) above(millicores uint64) bool {
	// Kubernetes converts a CPU request to shares as millicores * 1024 / 1000.
	limit := millicores * 1024 / 1000
	if r.isWeight {
		// Compare in weight space so the boundary matches the runtime's rounding exactly.
		limit = cpuSharesToWeight(limit)
	}
	return r.value > limit
}

func (r cgroupCPURequest) String() string {
	return fmt.Sprintf("%s %d", r.fileName, r.value)
}

// cpuSharesToWeight matches the conversion used by runc >= 1.3.2 and crun >= 1.23. See
// https://github.com/opencontainers/cgroups/blob/v0.0.4/utils.go. Older runtimes produce a lower weight for the same
// request, which makes thresholds more permissive.
func cpuSharesToWeight(shares uint64) uint64 {
	if shares <= 2 {
		return 1
	}
	if shares >= maxCPUShares {
		return 10000
	}
	l := math.Log2(float64(shares))
	return uint64(math.Ceil(math.Pow(10, (l*l+125*l)/612-7.0/34)))
}
