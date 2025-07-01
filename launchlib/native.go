// Copyright 2016 Palantir Technologies, Inc.
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
	"strings"
)

var (
	// Adapted from: https://github.com/palantir/sls-packaging/blob/develop/gradle-sls-packaging/src/main/java/com/palantir/gradle/dist/service/tasks/LaunchConfig.java
	// Not all JVM options are supported by native image
	AllowedNativeImageJVMOptions = []string{
		"-Djava.io.tmpdir=",
		"-Djna.tmpdir=",
		"-Dsun.net.inetaddr.ttl=",
	}
)

func hasMaximumHeapSizePercentOverride(args []string) bool {
	for _, arg := range args {
		if isMaximumHeapSizePercent(arg) {
			return true
		}
	}
	return false
}
func isMaximumHeapSizePercent(arg string) bool {
	return strings.HasPrefix(arg, "-XX:MaximumHeapSizePercent=")
}

func getNativeArgsFromJVMOpts(jvmOpts []string) []string {
	var nativeArgs = []string{}
	for _, arg := range jvmOpts {
		for _, allowed := range AllowedNativeImageJVMOptions {
			if strings.HasPrefix(arg, allowed) {
				nativeArgs = append(nativeArgs, arg)
			}
		}
	}
	return nativeArgs
}

// getNativeArgs filters out any heap-related options that are not supported by the current mode (container mode enabled/disabled), and adds -XX:MaximumHeapSizePercent using heapPercentage if applicable
func getNativeArgs(nativeArgs []string, customConfig *CustomLauncherConfig) []string {
	var filteredArgs []string

	if isEnvVarSet("CONTAINER") && !customConfig.DisableContainerSupport {
		// If container mode is enabled, filter out any heap size arguments (-Xmx, -Xms), if present
		filteredArgs = filterNativeHeapSizeArgs(nativeArgs)

		// If MaximumHeapSizePercent is not present in NativeImageArguments and container mode is not disabled, use heapPercentage for -XX:MaximumHeapSizePercent
		if !hasMaximumHeapSizePercentOverride(nativeArgs) && customConfig.HeapPercentage != nil {
			maxHeapSizeFlag := fmt.Sprintf("-XX:MaximumHeapSizePercent=%.2f", *customConfig.HeapPercentage)
			filteredArgs = append(filteredArgs, maxHeapSizeFlag)
		}
	} else {
		// If container mode is disabled, filter out any heap percentage arguments (-XX:MaximumHeapSizePercent), if present
		filteredArgs = filterNativeHeapPercentageArgs(nativeArgs)
	}
	return filteredArgs
}

func filterNativeHeapSizeArgs(nativeArgs []string) []string {
	var filtered []string
	for _, arg := range nativeArgs {
		if !isHeapSizeArg(arg) {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

func isNativeHeapPercentageArg(arg string) bool {
	return strings.HasPrefix(arg, "-XX:MaximumHeapSizePercent=")
}
func filterNativeHeapPercentageArgs(nativeArgs []string) []string {
	var filtered []string
	for _, arg := range nativeArgs {
		if !isNativeHeapPercentageArg(arg) {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}
