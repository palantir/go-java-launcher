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
	"io"
	"maps"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/pkg/errors"
)

const (
	TemplateDelimsOpen  = "{{"
	TemplateDelimsClose = "}}"
	// ExecPathBlackListRegex matches characters disallowed in paths we allow to be passed to exec()
	ExecPathBlackListRegex           = `[^\w.\/_\-]`
	BytesInMebibyte                  = 1048576
	defaultNativeImageExecutablePath = "service/bin/native-executable"
)

type ServiceCmds struct {
	Primary      *exec.Cmd
	SubProcesses map[string]*exec.Cmd
}

func CompileCmdsFromConfig(
	staticConfig *PrimaryStaticLauncherConfig, customConfig *PrimaryCustomLauncherConfig, loggers ServiceLoggers) (
	serviceCmds *ServiceCmds, err error) {
	serviceCmds = &ServiceCmds{
		SubProcesses: make(map[string]*exec.Cmd),
	}

	serviceCmds.Primary, err = compileCmdFromConfig(&staticConfig.StaticLauncherConfig, &customConfig.CustomLauncherConfig, &customConfig.CgroupsV1, loggers.PrimaryLogger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile command for primary command")
	}
	for name, subProcStatic := range staticConfig.SubProcesses {
		subProcCustom, ok := customConfig.SubProcesses[name]
		if !ok {
			return nil, errors.Errorf("no custom launcher config exists for subProcess config '%s'", name)
		}

		serviceCmds.SubProcesses[name], err = compileCmdFromConfig(&subProcStatic, &subProcCustom, &customConfig.CgroupsV1, loggers.SubProcessLogger(name))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to compile command for subProcess %s", name)
		}
	}
	return serviceCmds, nil
}

func compileCmdFromConfig(
	staticConfig *StaticLauncherConfig, customConfig *CustomLauncherConfig, cgroupsV1 *map[string]string, createLogger CreateLogger) (cmd *exec.Cmd, err error) {
	logger, err := createLogger()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create command compilation logger")
	}
	defer func() {
		if cErr := logger.Close(); cErr != nil && err == nil {
			err = errors.Wrapf(err, "unable to close command compilation logger")
		}
	}()
	_, _ = fmt.Fprintf(logger, "Launching with static configuration %v and custom configuration %v\n",
		*staticConfig, *customConfig)

	workingDir := getWorkingDir()
	_, _ = fmt.Fprintf(logger, "Working directory: %s\n", workingDir)

	var args []string
	var executable string
	var executableErr error

	if staticConfig.Type == "java" {
		var combinedJvmOpts []string
		combinedJvmOpts = append(combinedJvmOpts, staticConfig.JavaConfig.JvmOpts...)
		combinedJvmOpts = append(combinedJvmOpts, customConfig.JvmOpts...)

		// Handle experimental nativeImage execution mode before standard processing
		if customConfig.Experimental.ExecutionMode == ExecutionModeNative {
			nativeImageExecutablePath := customConfig.Experimental.NativeImageExecutablePath
			if nativeImageExecutablePath == "" {
				nativeImageExecutablePath = defaultNativeImageExecutablePath
			}
			executable, executableErr = verifyPathIsSafeForExec(nativeImageExecutablePath)
			if executableErr != nil {
				return nil, executableErr
			}
			args = append(args, executable) // 0th argument is the command itself

			// Add JVM options that are supported by native image (using the allowed list)
			args = append(args, getNativeArgsFromJVMOpts(combinedJvmOpts)...)
			// Filter out any heap-related options that are not supported by the current mode (container mode enabled/disabled). Add -XX:MaximumHeapSizePercent using heapPercentage if applicable
			args = append(args, getNativeArgs(customConfig.Experimental.NativeImageArguments, customConfig)...)
		} else {
			javaHome, javaHomeErr := getJavaHome(staticConfig.JavaConfig.JavaHome)
			if javaHomeErr != nil {
				return nil, javaHomeErr
			}
			_, _ = fmt.Fprintf(logger, "Using JAVA_HOME: %s\n", javaHome)

			classpath := joinClasspathEntries(absolutizeClasspathEntries(workingDir,
				staticConfig.JavaConfig.Classpath))
			_, _ = fmt.Fprintf(logger, "Classpath: %s\n", classpath)

			jvmOpts := createJvmOpts(combinedJvmOpts, customConfig, logger)

			executable, executableErr = verifyPathIsSafeForExec(path.Join(javaHome, "/bin/java"))
			if executableErr != nil {
				return nil, executableErr
			}
			args = append(args, executable) // 0th argument is the command itself
			args = append(args, jvmOpts...)
			args = append(args, "-classpath", classpath)
			args = append(args, staticConfig.JavaConfig.MainClass)
		}
	} else if staticConfig.Type == "executable" {
		executable, executableErr = verifyPathIsSafeForExec(staticConfig.Executable)
		if executableErr != nil {
			return nil, executableErr
		}
		args = append(args, executable) // 0th argument is the command itself
	} else {
		return nil, fmt.Errorf("can't launch type %v, this should have errored in config validation",
			staticConfig.Type)
	}

	args = append(args, staticConfig.Args...)
	if len(*cgroupsV1) > 0 {
		var cgexecArgs []string
		executable = "/bin/cgexec"

		cgexecArgs = append(cgexecArgs, executable)
		for controller, cgroup := range *cgroupsV1 {
			cgexecArgs = append(cgexecArgs, "-g", fmt.Sprintf("%s:%s", controller, cgroup))
		}
		cgexecArgs = append(cgexecArgs, args...)
		args = cgexecArgs
	}

	_, _ = fmt.Fprintf(logger, "Argument list to executable binary: %v\n\n", args)

	env := replaceEnvironmentVariables(merge(staticConfig.Env, customConfig.Env))

	return createCmd(executable, args, env)
}

func MkDirs(dirs []string, stdout io.Writer) error {
	isDirMatcher := regexp.MustCompile(`^[A-Za-z0-9]+(/[A-Za-z0-9]+)*$`).MatchString
	for _, dir := range dirs {
		if !isDirMatcher(dir) {
			return fmt.Errorf("Cannot create directory with non [A-Za-z0-9] characters: %s", dir)
		}

		_, _ = fmt.Fprintf(stdout, "Creating directory: %s\n", dir)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

// Returns true iff the given path is safe to be passed to exec(): must not contain funky characters and be a valid file
func verifyPathIsSafeForExec(execPath string) (string, error) {
	if unsafe, err := regexp.MatchString(ExecPathBlackListRegex, execPath); err != nil {
		return "", err
	} else if unsafe {
		return "", fmt.Errorf("Unsafe execution path: %q ", execPath)
	} else if _, statErr := os.Stat(execPath); statErr != nil {
		return "", statErr
	}

	return execPath, nil
}

// Returns explicitJavaHome if it is not the empty string, or the value of the JAVA_HOME environment variable otherwise.
// Panics if neither of them is set.
func getJavaHome(explicitJavaHome string) (string, error) {
	if explicitJavaHome == "" {
		return loadEnvVar("JAVA_HOME")
	} else if explicitJavaHome[0] == '$' {
		if len(explicitJavaHome) == 1 {
			return "", fmt.Errorf("javaHome set to just '$' is not allowed, please use a path or an env var name like $JAVA_11_HOME")
		}
		jh, err := loadEnvVar(explicitJavaHome[1:])
		if err != nil {
			return loadEnvVar("JAVA_HOME")
		}
		return jh, nil
	} else {
		return explicitJavaHome, nil
	}
}

func loadEnvVar(envVar string) (string, error) {
	javaHome := os.Getenv(envVar)
	if len(javaHome) == 0 {
		return "", fmt.Errorf("%s environment variable not set", envVar)
	}
	return javaHome, nil
}

func isEnvVarSet(envVar string) bool {
	_, set := os.LookupEnv(envVar)
	return set
}

func getWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}

// Prepends each of the given classpath entries with the given working directory.
func absolutizeClasspathEntries(workingDir string, relativeClasspathEntries []string) []string {
	absoluteClasspathEntries := make([]string, len(relativeClasspathEntries))
	for i, entry := range relativeClasspathEntries {
		absoluteClasspathEntries[i] = path.Join(workingDir, entry)
	}
	return absoluteClasspathEntries
}

func joinClasspathEntries(classpathEntries []string) string {
	return strings.Join(classpathEntries, ":")
}

func createCmd(executable string, args []string, customEnv map[string]string) (*exec.Cmd, error) {
	env := os.Environ()
	for key, value := range customEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd := &exec.Cmd{
		Path: executable,
		Args: args,
		Env:  env,
	}

	return cmd, nil
}

// Performs replacement of all replaceable values in env, returning a new
// map, with the same keys as env, but possibly changed values.
func replaceEnvironmentVariables(env map[string]string) map[string]string {
	replacer := createReplacer()

	returnMap := make(map[string]string)
	for key, value := range env {
		returnMap[key] = replacer.Replace(value)
	}

	return returnMap
}

// copy all the keys and values from overrideMap into origMap. If a key already
// exists in origMap, its value is overridden.
func merge(origMap, overrideMap map[string]string) map[string]string {
	if len(overrideMap) == 0 {
		return origMap
	}

	returnMap := make(map[string]string)
	maps.Copy(returnMap, origMap)
	maps.Copy(returnMap, overrideMap)
	return returnMap
}

func createReplacer() *strings.Replacer {
	return strings.NewReplacer(
		delim("CWD"), getWorkingDir(),
	)
}

func delim(str string) string {
	return fmt.Sprintf("%s%s%s", TemplateDelimsOpen, str, TemplateDelimsClose)
}

func createJvmOpts(combinedJvmOpts []string, customConfig *CustomLauncherConfig, logger io.WriteCloser) []string {
	if isEnvVarSet("CONTAINER") && !customConfig.DisableContainerSupport && !hasMaxRAMOverride(combinedJvmOpts) {
		_, _ = fmt.Fprintln(logger, "Container support enabled")
		cgroupMemoryLimitInBytes, err := getCGroupMemoryLimitInBytes()
		if err != nil {
			// When we fail to get the memory limit from the cgroups files, fallback to using percentage-based heap
			// sizing. While this method doesn't take into account the per-processor memory offset, it is supported
			// by all platforms using Java.
			_, _ = fmt.Fprintf(logger, "Failed to get cgroup memory limit, falling back to percentage-based heap sizing: %v\n", err)
			return filterHeapSizeArgs(combinedJvmOpts, customConfig.HeapPercentage)
		}
		if cgroupMemoryLimitInBytes > 1_000_000*BytesInMebibyte {
			// When the memory limit is unusually high (defined to be over 1TB), revert to percentage-based heap
			// sizing. This handles the edge case where the cgroups memory limit is set to an arbitrary large value.
			_, _ = fmt.Fprintf(logger, "Cgroup memory limit unusually high (%d bytes), falling back to percentage-based heap sizing\n", cgroupMemoryLimitInBytes)
			return filterHeapSizeArgs(combinedJvmOpts, customConfig.HeapPercentage)
		}
		return filterHeapSizeArgsV2(combinedJvmOpts, customConfig.HeapPercentage, cgroupMemoryLimitInBytes, heapSizeOptions{
			allowHeapShrink:  customConfig.AllowHeapShrink || customConfig.Experimental.AllowHeapShrink,
			minHeapFreeRatio: customConfig.MinHeapFreeRatio,
			maxHeapFreeRatio: customConfig.MaxHeapFreeRatio,
		})
	}

	if isEnvVarSet("CONTAINER") {
		if customConfig.DisableContainerSupport {
			_, _ = fmt.Fprintln(logger, "Container support disabled in launcher-custom.yml")
		} else if hasMaxRAMOverride(combinedJvmOpts) {
			_, _ = fmt.Fprintln(logger, "Container support disabled: -XX:MaxRAM override present")
		}
	}

	return combinedJvmOpts
}

func filterHeapSizeArgs(args []string, heapPercentage *float64) []string {
	var filtered []string
	var hasMaxRAMPercentage, hasInitialRAMPercentage bool
	for _, arg := range args {
		if !isHeapSizeArg(arg) {
			filtered = append(filtered, arg)
		}

		if isMaxRAMPercentage(arg) {
			hasMaxRAMPercentage = true
		} else if isInitialRAMPercentage(arg) {
			hasInitialRAMPercentage = true
		}
	}

	if !hasInitialRAMPercentage && !hasMaxRAMPercentage {
		percentage := 75.0
		if heapPercentage != nil {
			percentage = *heapPercentage
		}
		filtered = append(filtered, fmt.Sprintf("-XX:InitialRAMPercentage=%.1f", percentage))
		filtered = append(filtered, fmt.Sprintf("-XX:MaxRAMPercentage=%.1f", percentage))
	}
	return filtered
}

func getCGroupMemoryLimitInBytes() (uint64, error) {
	memoryLimit, err := NewCGroupMemoryLimit(os.DirFS("/"))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get cgroup memory limit")
	}
	cgroupMemoryLimitInBytes, err := memoryLimit.MemoryLimitInBytes()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get cgroup memory limit")
	}
	return cgroupMemoryLimitInBytes, nil
}

// heapSizeOptions carries the launcher-managed heap tuning flags derived from the custom launcher config. Dedicated
// fields allow installation-level heap ratio overrides without replacing the product-level jvmOpts list.
type heapSizeOptions struct {
	allowHeapShrink  bool
	minHeapFreeRatio *int
	maxHeapFreeRatio *int
}

func filterHeapSizeArgsV2(args []string, heapPercentage *float64, cgroupMemoryLimitInBytes uint64, opts heapSizeOptions) []string {
	var filtered []string
	var hasMaxRAMPercentage, hasInitialRAMPercentage bool
	for _, arg := range args {
		if isHeapSizeArg(arg) {
			continue
		}
		if opts.allowHeapShrink && isAlwaysPreTouch(arg) {
			continue
		}
		// When a ratio is configured, drop any equivalent from jvmOpts so the config value wins.
		if opts.minHeapFreeRatio != nil && isMinHeapFreeRatio(arg) {
			continue
		}
		if opts.maxHeapFreeRatio != nil && isMaxHeapFreeRatio(arg) {
			continue
		}
		filtered = append(filtered, arg)

		if isMaxRAMPercentage(arg) {
			hasMaxRAMPercentage = true
		} else if isInitialRAMPercentage(arg) {
			hasInitialRAMPercentage = true
		}
	}

	if !hasInitialRAMPercentage && !hasMaxRAMPercentage {
		jvmHeapSizeInBytes := ComputeJVMHeapSizeInBytes(runtime.NumCPU(), cgroupMemoryLimitInBytes, heapPercentage)
		if !opts.allowHeapShrink {
			filtered = append(filtered, fmt.Sprintf("-Xms%d", jvmHeapSizeInBytes))
		}
		filtered = append(filtered, fmt.Sprintf("-Xmx%d", jvmHeapSizeInBytes))
	}

	// Heap free ratios are orthogonal to the heap sizing method, so append them regardless of whether the RAM
	// percentage overrides above are present.
	if opts.minHeapFreeRatio != nil {
		filtered = append(filtered, fmt.Sprintf("-XX:MinHeapFreeRatio=%d", *opts.minHeapFreeRatio))
	}
	if opts.maxHeapFreeRatio != nil {
		filtered = append(filtered, fmt.Sprintf("-XX:MaxHeapFreeRatio=%d", *opts.maxHeapFreeRatio))
	}
	return filtered
}

func hasMaxRAMOverride(args []string) bool {
	return slices.ContainsFunc(args, isMaxRAM)
}

func isMaxRAM(arg string) bool {
	return strings.HasPrefix(arg, "-XX:MaxRAM=")
}

func isHeapSizeArg(arg string) bool {
	return strings.HasPrefix(arg, "-Xmx") || strings.HasPrefix(arg, "-Xms")
}

func isAlwaysPreTouch(arg string) bool {
	return arg == "-XX:+AlwaysPreTouch"
}

func isMinHeapFreeRatio(arg string) bool {
	return strings.HasPrefix(arg, "-XX:MinHeapFreeRatio=")
}

func isMaxHeapFreeRatio(arg string) bool {
	return strings.HasPrefix(arg, "-XX:MaxHeapFreeRatio=")
}

func isMaxRAMPercentage(arg string) bool {
	return strings.HasPrefix(arg, "-XX:MaxRAMPercentage=")
}

func isInitialRAMPercentage(arg string) bool {
	return strings.HasPrefix(arg, "-XX:InitialRAMPercentage=")
}

// ComputeJVMHeapSizeInBytes By default, compute the heap size to be 75% of the heap minus 3mb per processor, with a minimum value
// of 50% of the heap. If heapPercentage is provided, use that percentage instead with no processor adjustment.
func ComputeJVMHeapSizeInBytes(hostProcessorCount int, cgroupMemoryLimitInBytes uint64, heapPercentage *float64) uint64 {
	var memoryLimit = float64(cgroupMemoryLimitInBytes)

	if heapPercentage != nil {
		return uint64(*heapPercentage / 100 * memoryLimit)
	}

	var processorAdjustment = 3 * BytesInMebibyte * float64(hostProcessorCount)
	var computedHeapSize = max(0.5*memoryLimit, 0.75*memoryLimit-processorAdjustment)
	return uint64(computedHeapSize)
}
