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
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	trimNativeHeapEnvVar      = "LAUNCHER_EXPERIMENTAL_TRIM_NATIVE_HEAP"
	trimNativeHeapIntervalArg = "-XX:TrimNativeHeapInterval="
	// The default trim interval. Benchmarks showed 5s noticeably lowers peak RSS during startup (class loading and JIT
	// compilation) compared to 15s, with steady-state peaks within a few MiB of each other.
	trimNativeHeapDefaultIntervalSeconds = 5
	// TrimNativeHeapInterval is a uint in milliseconds, so this is the largest whole number of seconds it can hold.
	trimNativeHeapMaxIntervalSeconds = math.MaxUint32 / 1000
	// Periodic trimming is only applied to services with a CPU request and heap at or below these limits. The benefit
	// is a roughly fixed number of MiB per JVM, which only matters relative to a small container.
	trimNativeHeapMaxCPUCores  = 4
	trimNativeHeapMaxHeapBytes = 8 * 1024 * BytesInMebibyte
)

func trimNativeHeapEnabled(customConfig *CustomLauncherConfig) (bool, error) {
	if value, set := os.LookupEnv(trimNativeHeapEnvVar); set && value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return false, errors.Wrapf(err, "invalid %s value", trimNativeHeapEnvVar)
		}
		return enabled, nil
	}
	return customConfig.Experimental.TrimNativeHeap, nil
}

// trimNativeHeapIntervalSeconds returns the configured trim interval, or the default if unset. The configured value
// is validated when the custom config is parsed.
func trimNativeHeapIntervalSeconds(customConfig *CustomLauncherConfig) int {
	if interval := customConfig.Experimental.TrimNativeHeapIntervalSeconds; interval != nil {
		return *interval
	}
	return trimNativeHeapDefaultIntervalSeconds
}

// ensureTrimNativeHeap appends -XX:TrimNativeHeapInterval for small services so the JVM periodically returns memory
// freed into glibc malloc arenas to the OS. heapBytes is the max heap size chosen by cgroup-based heap sizing.
func ensureTrimNativeHeap(
	args []string, filesystem fs.FS, heapBytes uint64, intervalSeconds int, logger io.Writer,
) []string {
	for _, arg := range args {
		if strings.HasPrefix(arg, trimNativeHeapIntervalArg) {
			return args
		}
	}
	if heapBytes > trimNativeHeapMaxHeapBytes {
		_, _ = fmt.Fprintf(logger, "Not trimming native heap: max heap %d bytes is above %d\n", heapBytes, uint64(trimNativeHeapMaxHeapBytes))
		return args
	}
	var requestedCores uint
	counter, err := newCGroupProcessorCounter(filesystem)
	if err == nil {
		requestedCores, err = counter.ProcessorCount()
	}
	if err != nil {
		_, _ = fmt.Fprintf(logger, "Not trimming native heap: failed to get cgroup CPU request: %v\n", err)
		return args
	}
	if requestedCores > trimNativeHeapMaxCPUCores {
		_, _ = fmt.Fprintf(logger, "Not trimming native heap: CPU request of %d cores is above %d\n", requestedCores, trimNativeHeapMaxCPUCores)
		return args
	}
	return append(args, fmt.Sprintf("%s%d", trimNativeHeapIntervalArg, intervalSeconds*1000))
}
