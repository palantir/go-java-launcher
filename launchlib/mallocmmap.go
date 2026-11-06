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
	pinMallocMmapThresholdEnvVar = "LAUNCHER_EXPERIMENTAL_PIN_MALLOC_MMAP_THRESHOLD"
	glibcTunablesEnvVar          = "GLIBC_TUNABLES"
	mallocMmapThresholdTunable   = "glibc.malloc.mmap_threshold"
	// glibc's own initial threshold. Setting it explicitly turns off glibc's dynamic threshold, which otherwise rises
	// to the size of every large mmapped block that is freed, so later blocks of that size stay in the malloc arenas.
	mallocMmapThresholdBytes = 128 * 1024
	// Only applied to services with a CPU request and heap at or below these limits.
	pinMallocMmapThresholdMaxCPUMillicores = 200
	pinMallocMmapThresholdMaxHeapBytes     = 1024 * BytesInMebibyte
	maxCPUShares                           = 262144
)

func pinMallocMmapThresholdEnabled(customConfig *CustomLauncherConfig) (bool, error) {
	if value, set := os.LookupEnv(pinMallocMmapThresholdEnvVar); set && value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return false, errors.Wrapf(err, "invalid %s value", pinMallocMmapThresholdEnvVar)
		}
		return enabled, nil
	}
	return customConfig.Experimental.PinMallocMmapThreshold, nil
}

// ensureMallocMmapThreshold adds glibc.malloc.mmap_threshold to GLIBC_TUNABLES for small services, keeping any other
// tunables. env holds the launcher-configured environment, which overrides the inherited process environment.
func ensureMallocMmapThreshold(env map[string]string, filesystem fs.FS, heapBytes uint64, logger io.Writer) map[string]string {
	if heapBytes > pinMallocMmapThresholdMaxHeapBytes {
		_, _ = fmt.Fprintf(logger, "Not pinning malloc mmap threshold: max heap %d bytes is above %d\n", heapBytes, uint64(pinMallocMmapThresholdMaxHeapBytes))
		return env
	}
	above, request, err := cgroupCPURequestAbove(filesystem, pinMallocMmapThresholdMaxCPUMillicores)
	if err != nil {
		_, _ = fmt.Fprintf(logger, "Not pinning malloc mmap threshold: failed to get cgroup CPU request: %v\n", err)
		return env
	}
	if above {
		_, _ = fmt.Fprintf(logger, "Not pinning malloc mmap threshold: CPU request is above %dm: %s\n", pinMallocMmapThresholdMaxCPUMillicores, request)
		return env
	}
	tunables, configured := env[glibcTunablesEnvVar]
	if !configured {
		tunables = os.Getenv(glibcTunablesEnvVar)
	}
	for _, tunable := range strings.Split(tunables, ":") {
		if strings.HasPrefix(tunable, mallocMmapThresholdTunable+"=") {
			return env
		}
	}
	setting := fmt.Sprintf("%s=%d", mallocMmapThresholdTunable, mallocMmapThresholdBytes)
	if tunables != "" {
		setting = tunables + ":" + setting
	}
	return merge(env, map[string]string{glibcTunablesEnvVar: setting})
}

// cgroupCPURequestAbove reports whether the container CPU request is above maxMillicores, using cgroup v1 cpu.shares
// or cgroup v2 cpu.weight. The returned string describes the observed value for logging.
func cgroupCPURequestAbove(filesystem fs.FS, maxMillicores uint64) (bool, string, error) {
	isCGroupV2, err := IsCGroupV2(filesystem)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to determine cgroup version")
	}
	pather, fileName := NewCGroupV1Pather(filesystem), cpuSharesName
	if isCGroupV2 {
		pather, fileName = NewCGroupV2Pather(), cpuWeightName
	}
	cpuCGroupPath, err := pather.Path(cpuGroupName)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to get cpu cgroup path")
	}
	contents, err := fs.ReadFile(filesystem, convertToFSPath(filepath.Join(cpuCGroupPath, fileName)))
	if err != nil {
		return false, "", errors.Wrapf(err, "unable to read %s", fileName)
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil || value == 0 || (isCGroupV2 && value > 10000) {
		return false, "", errors.Errorf("invalid %s value: %q", fileName, strings.TrimSpace(string(contents)))
	}
	// Kubernetes converts a CPU request to shares as millicores * 1024 / 1000.
	limit := maxMillicores * 1024 / 1000
	if isCGroupV2 {
		// Compare in weight space so the boundary matches the runtime's rounding exactly.
		limit = cpuSharesToWeight(limit)
	}
	return value > limit, fmt.Sprintf("%s %d (limit %d)", fileName, value, limit), nil
}

// cpuSharesToWeight matches the conversion used by runc >= 1.3.2 and crun >= 1.23. See
// https://github.com/opencontainers/cgroups/blob/v0.0.4/utils.go. Older runtimes produce a lower weight for the same
// request, which makes the threshold more permissive.
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
