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
	"math/bits"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	limitCompilerThreadsEnvVar = "LAUNCHER_EXPERIMENTAL_LIMIT_COMPILER_THREADS"
	// Like the GC thread limit, size for 8 processors per requested core.
	millicoresPerCompilerProcessor = 1000 / 8
)

func limitCompilerThreadsEnabled(customConfig *CustomLauncherConfig) (bool, error) {
	if value, set := os.LookupEnv(limitCompilerThreadsEnvVar); set && value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return false, errors.Wrapf(err, "invalid %s value", limitCompilerThreadsEnvVar)
		}
		return enabled, nil
	}
	return customConfig.Experimental.LimitCompilerThreads, nil
}

// ensureCompilerThreads adds -XX:CICompilerCount using the JVM's own sizing formula for min(8 * CPU request, available
// processors) rather than the available processors, when that is lower than the JVM default.
func ensureCompilerThreads(args []string, filesystem fs.FS, hostProcessors int, logger io.Writer) []string {
	for _, arg := range args {
		// Explicit compiler or processor settings take precedence. A smaller code cache can lower the JVM default.
		if strings.HasPrefix(arg, "-XX:CICompilerCount=") || strings.HasSuffix(arg, "CICompilerCountPerCPU") ||
			strings.HasPrefix(arg, "-XX:ActiveProcessorCount=") || strings.HasPrefix(arg, "-XX:ReservedCodeCacheSize=") ||
			arg == "-Xint" {
			return args
		}
	}
	request, err := readCgroupCPURequest(filesystem)
	if err != nil {
		_, _ = fmt.Fprintf(logger, "Failed to get cgroup CPU request, using JVM default compiler threads: %v\n", err)
		return args
	}
	counter, err := newCGroupProcessorCounter(filesystem)
	if err == nil {
		hostProcessors, err = counter.clampToCPUQuota(hostProcessors)
	}
	if err != nil {
		_, _ = fmt.Fprintf(logger, "Failed to get cgroup CPU quota, using JVM default compiler threads: %v\n", err)
		return args
	}
	processors := 1
	for processors < hostProcessors && request.above(uint64(processors)*millicoresPerCompilerProcessor) {
		processors++
	}
	count := compilerCount(processors)
	if count >= compilerCount(hostProcessors) {
		return args
	}
	_, _ = fmt.Fprintf(logger, "Setting CICompilerCount to %d based on %s\n", count, request)
	return append(args, fmt.Sprintf("-XX:CICompilerCount=%d", count))
}

// compilerCount matches the JVM's CICompilerCountPerCPU sizing in CompilationPolicy::initialize.
func compilerCount(processors int) int {
	logCPU := bits.Len(uint(processors)) - 1
	logLogCPU := bits.Len(uint(max(logCPU, 1))) - 1
	return max(logCPU*logLogCPU*3/2, 2)
}
