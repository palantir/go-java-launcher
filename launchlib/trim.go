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
	"bufio"
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
	trimNativeHeapEnvVar      = "LAUNCHER_EXPERIMENTAL_TRIM_NATIVE_HEAP"
	trimNativeHeapIntervalArg = "-XX:TrimNativeHeapInterval="
	trimNativeHeapIntervalMs  = 5000
	// Periodic trimming is only applied to services with a CPU request and heap at or below these limits. The benefit
	// is a roughly fixed number of MiB per JVM, which only matters relative to a small container.
	trimNativeHeapMaxCPUMillicores = 4000
	trimNativeHeapMaxHeapBytes     = 8 * 1024 * BytesInMebibyte

	cpuCGroupName         = CGroupName("cpu")
	cgroupV1CPUSharesFile = "cpu.shares"
	cgroupV2CPUWeightFile = "cpu.weight"
	cgroupMaxShares       = 262144
)

func trimNativeHeapEnabled(customConfig *CustomLauncherConfig, logger io.Writer) bool {
	value, set := os.LookupEnv(trimNativeHeapEnvVar)
	if !set {
		return customConfig.Experimental.TrimNativeHeap
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		_, _ = fmt.Fprintf(logger, "Ignoring invalid %s value %q\n", trimNativeHeapEnvVar, value)
		return customConfig.Experimental.TrimNativeHeap
	}
	return enabled
}

// ensureTrimNativeHeap appends -XX:TrimNativeHeapInterval for small services so the JVM periodically returns memory
// freed into glibc malloc arenas to the OS. args must already contain the final heap size flags.
func ensureTrimNativeHeap(args []string, filesystem fs.FS, javaHome string, logger io.Writer) []string {
	var heapBytes uint64
	var hasHeapBytes, ignoresUnrecognizedOptions bool
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, trimNativeHeapIntervalArg):
			return args
		case arg == "-XX:+IgnoreUnrecognizedVMOptions":
			ignoresUnrecognizedOptions = true
		case strings.HasPrefix(arg, "-Xmx"):
			if size, err := parseJVMMemorySize(arg[len("-Xmx"):]); err == nil {
				heapBytes, hasHeapBytes = size, true
			}
		}
	}
	if !hasHeapBytes {
		_, _ = fmt.Fprintln(logger, "Not trimming native heap: unable to determine max heap size")
		return args
	}
	if heapBytes > trimNativeHeapMaxHeapBytes {
		_, _ = fmt.Fprintf(logger, "Not trimming native heap: max heap %d bytes is above %d\n", heapBytes, uint64(trimNativeHeapMaxHeapBytes))
		return args
	}
	cpuAbove, cpuRequest, err := cgroupCPURequestAbove(filesystem, trimNativeHeapMaxCPUMillicores)
	if err != nil {
		_, _ = fmt.Fprintf(logger, "Not trimming native heap: failed to get cgroup CPU request: %v\n", err)
		return args
	}
	if cpuAbove {
		_, _ = fmt.Fprintf(logger, "Not trimming native heap: CPU request is above %dm: %s\n", trimNativeHeapMaxCPUMillicores, cpuRequest)
		return args
	}
	// Older JVMs fail to start on unrecognized -XX options; the flag exists on Java 17+.
	if !ignoresUnrecognizedOptions {
		major, err := javaMajorVersion(filesystem, javaHome)
		if err != nil || major < 17 {
			_, _ = fmt.Fprintf(logger, "Not trimming native heap: unable to confirm Java 17+ (version %d, error %v)\n", major, err)
			return args
		}
	}
	return append(args, fmt.Sprintf("%s%d", trimNativeHeapIntervalArg, trimNativeHeapIntervalMs))
}

// cgroupCPURequestAbove reports whether the container CPU request is above maxMillicores, using cgroup v1 cpu.shares
// or cgroup v2 cpu.weight. The returned string describes the observed value for logging.
func cgroupCPURequestAbove(filesystem fs.FS, maxMillicores uint64) (bool, string, error) {
	isCGroupV2, err := IsCGroupV2(filesystem)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to determine cgroup version")
	}
	pather, fileName := NewCGroupV1Pather(filesystem), cgroupV1CPUSharesFile
	if isCGroupV2 {
		pather, fileName = NewCGroupV2Pather(), cgroupV2CPUWeightFile
	}
	cpuCGroupPath, err := pather.Path(cpuCGroupName)
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
	if shares >= cgroupMaxShares {
		return 10000
	}
	l := math.Log2(float64(shares))
	return uint64(math.Ceil(math.Pow(10, (l*l+125*l)/612-7.0/34)))
}

// parseJVMMemorySize parses a JVM memory size such as 512m, 2g, 1048576k or 1073741824.
func parseJVMMemorySize(value string) (uint64, error) {
	multiplier := uint64(1)
	if n := len(value); n > 0 {
		switch value[n-1] {
		case 'k', 'K':
			multiplier = 1024
		case 'm', 'M':
			multiplier = BytesInMebibyte
		case 'g', 'G':
			multiplier = 1024 * BytesInMebibyte
		case 't', 'T':
			multiplier = 1024 * 1024 * BytesInMebibyte
		}
		if multiplier != 1 {
			value = value[:n-1]
		}
	}
	size, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid memory size %q", value)
	}
	return size * multiplier, nil
}

// javaMajorVersion reads JAVA_VERSION from the release file of the given Java home.
func javaMajorVersion(filesystem fs.FS, javaHome string) (int, error) {
	file, err := filesystem.Open(convertToFSPath(filepath.Join(javaHome, "release")))
	if err != nil {
		return 0, errors.Wrap(err, "failed to open release file")
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "JAVA_VERSION=") {
			continue
		}
		version := strings.Trim(strings.TrimPrefix(line, "JAVA_VERSION="), `"`)
		// Java 8 and earlier report 1.x.
		version = strings.TrimPrefix(version, "1.")
		major, _, _ := strings.Cut(version, ".")
		return strconv.Atoi(major)
	}
	return 0, errors.New("JAVA_VERSION not found in release file")
}
