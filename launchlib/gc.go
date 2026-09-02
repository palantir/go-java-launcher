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
	"strings"
)

func applyZGCCanaryJvmOpts(
	combinedJvmOpts []string,
	hostnameSuffix string,
	hostnameFunc func() (string, error),
	logger io.Writer,
) []string {
	if hostnameSuffix == "" {
		return combinedJvmOpts
	}

	hostname, err := hostnameFunc()
	if err != nil {
		_, _ = fmt.Fprintf(logger, "Failed to resolve hostname for experimental ZGC canary; retaining configured JVM options: %v\n", err)
		return combinedJvmOpts
	}
	if !strings.HasSuffix(hostname, hostnameSuffix) {
		_, _ = fmt.Fprintf(logger, "ZGC canary hostname %q does not match suffix %q; retaining configured GC profile\n", hostname, hostnameSuffix)
		return combinedJvmOpts
	}

	_, _ = fmt.Fprintf(logger, "ZGC canary hostname %q matches suffix %q; selecting SLS Packaging response-time profile\n",
		hostname, hostnameSuffix)
	return useSLSPackagingZGCProfile(combinedJvmOpts)
}

func useSLSPackagingZGCProfile(jvmOpts []string) []string {
	filtered := make([]string, 0, len(jvmOpts)+3)
	for _, opt := range jvmOpts {
		if !isReplacedGCProfileJvmOpt(opt) {
			filtered = append(filtered, opt)
		}
	}

	// Keep these aligned with the JDK 21+ response-time profile in sls-packaging's GcProfile.ResponseTime:
	// https://github.com/palantir/sls-packaging/blob/develop/gradle-sls-packaging/src/main/java/com/palantir/gradle/dist/service/gc/GcProfile.java
	// ZGenerational is ignored by JDK 24+, but retained while sls-packaging emits it.
	return append(filtered,
		"-XX:+UseZGC",
		"-XX:+ZGenerational",
		"-XX:+ExplicitGCInvokesConcurrent")
}

func isReplacedGCProfileJvmOpt(opt string) bool {
	if strings.HasPrefix(opt, "-XX:MaxGCPauseMillis=") {
		return true
	}

	const xxBooleanOptionPrefix = "-XX:"
	if !strings.HasPrefix(opt, xxBooleanOptionPrefix) || len(opt) <= len(xxBooleanOptionPrefix) {
		return false
	}
	if opt[len(xxBooleanOptionPrefix)] != '+' && opt[len(xxBooleanOptionPrefix)] != '-' {
		return false
	}
	option := opt[len(xxBooleanOptionPrefix)+1:]
	switch option {
	case "UseSerialGC",
		"UseParallelGC",
		"UseParallelOldGC",
		"UseParNewGC",
		"UseConcMarkSweepGC",
		"UseG1GC",
		"UseZGC",
		"UseShenandoahGC",
		"UseEpsilonGC",
		"UseNUMA",
		"ZGenerational",
		"ExplicitGCInvokesConcurrent":
		return true
	default:
		return false
	}
}
