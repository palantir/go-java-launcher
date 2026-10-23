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
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJavaHome_usesJAVA_HOMEbydefault(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	require.NoError(t, os.Setenv("JAVA_HOME", "foo"))

	javaHome, javaHomeErr := getJavaHome("")
	assert.Equal(t, "foo", javaHome, "JAVA_HOME incorrect")
	assert.NoError(t, javaHomeErr, "getJavaHome correctly returns nil")
	javaHome, javaHomeErr = getJavaHome("explicit javahome")
	assert.Equal(t, "explicit javahome", javaHome, "JAVA_HOME incorrect")
	assert.NoError(t, javaHomeErr, "getJavaHome correctly returns nil")

	require.NoError(t, os.Setenv("JAVA_HOME", originalJavaHome))
}

func TestGetJavaHome_allowsReadingOtherEnvVar(t *testing.T) {
	original := os.Getenv("SOME_VAR")
	defer func() { require.NoError(t, os.Setenv("SOME_VAR", original)) }()

	require.NoError(t, os.Setenv("SOME_VAR", "foo"))

	javaHome, javaHomeErr := getJavaHome("$SOME_VAR")
	assert.NoError(t, javaHomeErr)
	assert.Equal(t, "foo", javaHome)
}

func TestGetJavaHome_defaultsToJavaHomeWhenUndefined(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	require.NoError(t, os.Setenv("JAVA_HOME", "foo"))

	javaHome, javaHomeErr := getJavaHome("$SOME_VAR")
	assert.NoError(t, javaHomeErr)
	assert.Equal(t, "foo", javaHome)

	require.NoError(t, os.Setenv("JAVA_HOME", originalJavaHome))
}

func TestSetCustomEnvironment(t *testing.T) {
	originalEnv := make(map[string]string)
	customEnv := map[string]string{
		"SOME_PATH": "{{CWD}}/full/path",
		"SOME_VAR":  "CUSTOM_VAR",
	}

	env := replaceEnvironmentVariables(merge(originalEnv, customEnv))
	cwd := getWorkingDir()

	if got, ok := env["SOME_PATH"]; ok {
		want := fmt.Sprintf("%s/full/path", cwd)
		assert.Equal(t, want, got, "SOME_PATH environment variable incorrect")
	} else {
		t.Errorf("Expected SOME_PATH to exist in map but it didn't")
	}

	if got, ok := env["SOME_VAR"]; ok {
		assert.Equal(t, "CUSTOM_VAR", got, "SOME_VAR environment variable incorrect")
	} else {
		t.Errorf("Expected CUSTOM_VAR to exist in map, but it didn't")
	}

	args := []string{"arg1", "arg2"}
	cmd, err := createCmd("my-command", args, env)
	assert.NoError(t, err)

	assert.Equal(t, "my-command", cmd.Path, "Command to be run was incorrect")
	assert.Equal(t, args, cmd.Args)

	startingEnv := os.Environ()
	wantEnv := append(startingEnv, []string{
		fmt.Sprintf("SOME_PATH=%s/full/path", cwd),
		"SOME_VAR=CUSTOM_VAR",
	}...)

	sort.Strings(cmd.Env)
	sort.Strings(wantEnv)
	assert.Equal(t, wantEnv, cmd.Env)
}

func TestUnknownVariablesAreNotExpanded(t *testing.T) {
	originalEnv := make(map[string]string)
	customEnv := map[string]string{
		"SOME_VAR": "{{FOO}}",
	}

	env := replaceEnvironmentVariables(merge(originalEnv, customEnv))
	if got, ok := env["SOME_VAR"]; ok {
		assert.Equal(t, "{{FOO}}", got, "SOME_VAR environment variable incorrect")
	} else {
		t.Errorf("Expected SOME_VAR to exist in map, but it didn't")
	}
}

func TestKeysAreNotExpanded(t *testing.T) {
	originalEnv := make(map[string]string)
	customEnv := map[string]string{
		"{{CWD}}": "Value",
	}

	env := replaceEnvironmentVariables(merge(originalEnv, customEnv))
	if got, ok := env["{{CWD}}"]; ok {
		assert.Equal(t, "Value", got, "%%CWD%% environment variable incorrect")
	} else {
		t.Errorf("Expected %%CWD%% to exist in map and not be expanded, but it didn't")
	}
}

func TestMkdirChecksDirectorySyntax(t *testing.T) {
	err := MkDirs([]string{"abc/def1"}, os.Stdout)
	assert.NoError(t, err)

	err = MkDirs([]string{"abc"}, os.Stdout)
	assert.NoError(t, err)

	require.NoError(t, os.RemoveAll("abc"))

	badCases := []string{
		"^&*",
		"abc//def",
		"abc/../def",
	}
	for _, dir := range badCases {
		err = MkDirs([]string{dir}, os.Stdout)
		assert.EqualError(t, err, "Cannot create directory with non [A-Za-z0-9] characters: "+dir)
	}
}

func TestCreateJvmOpts_GCThreadsActivationGates(t *testing.T) {
	t.Setenv(limitGCThreadsEnvVar, "")
	require.NoError(t, os.Unsetenv(limitGCThreadsEnvVar))
	for _, tc := range []struct {
		name             string
		enabled          bool
		envOverride      *string
		collector        string
		container        bool
		disableContainer bool
		maxRAMOverride   bool
		processorCount   string
		wantActive       bool
		wantError        bool
	}{
		{name: "explicit G1", enabled: true, collector: "-XX:+UseG1GC", container: true, wantActive: true},
		{name: "no explicit collector is inert", enabled: true, container: true},
		{name: "explicit non-G1", enabled: true, collector: "-XX:+UseZGC", container: true},
		{name: "explicitly disabled G1", enabled: true, collector: "-XX:-UseG1GC", container: true},
		{name: "flag disabled", collector: "-XX:+UseG1GC", container: true},
		{name: "outside container", enabled: true, collector: "-XX:+UseG1GC"},
		{name: "container support disabled", enabled: true, collector: "-XX:+UseG1GC", container: true, disableContainer: true},
		{name: "MaxRAM override", enabled: true, collector: "-XX:+UseG1GC", container: true, maxRAMOverride: true},
		{name: "active processor override", enabled: true, collector: "-XX:+UseG1GC", container: true, processorCount: "2"},
		{name: "env cannot override explicit processor count", envOverride: new("true"), collector: "-XX:+UseG1GC", container: true, processorCount: "2"},
		{name: "env enables disabled config", envOverride: new("true"), collector: "-XX:+UseG1GC", container: true, wantActive: true},
		{name: "env disables enabled config", enabled: true, envOverride: new("false"), collector: "-XX:+UseG1GC", container: true},
		{name: "empty env preserves disabled config", envOverride: new(""), collector: "-XX:+UseG1GC", container: true},
		{name: "empty env preserves enabled config", enabled: true, envOverride: new(""), collector: "-XX:+UseG1GC", container: true, wantActive: true},
		{name: "invalid env rejects disabled config", envOverride: new("invalid"), container: true, wantError: true},
		{name: "invalid env rejects enabled config", enabled: true, envOverride: new("invalid"), container: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envOverride != nil {
				t.Setenv(limitGCThreadsEnvVar, *tc.envOverride)
			}
			t.Setenv("CONTAINER", "")
			if !tc.container {
				require.NoError(t, os.Unsetenv("CONTAINER"))
			}
			args := []string{"-XX:MaxRAMPercentage=75.0"}
			if tc.collector != "" {
				args = append(args, tc.collector)
			}
			if tc.maxRAMOverride {
				args = append(args, "-XX:MaxRAM=1024")
			}
			if tc.processorCount != "" {
				args = append(args, "-XX:ActiveProcessorCount="+tc.processorCount)
			}
			config := CustomLauncherConfig{
				DisableContainerSupport: tc.disableContainer,
				Experimental:            ExperimentalLauncherConfig{LimitGCThreads: tc.enabled},
			}
			var log bytes.Buffer
			got, err := createJvmOpts(args, &config, &NoopClosingWriter{&log})
			assert.Equal(t, tc.enabled, config.Experimental.LimitGCThreads, "environment override must not mutate config")
			if tc.wantError {
				require.ErrorContains(t, err, "invalid "+limitGCThreadsEnvVar+" value")
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			if tc.wantActive {
				// Depending on the host, the limiter either appends a thread count or logs a cgroup
				// read failure. Both outcomes prove it ran; TestEnsureGCThreads checks exact sizing.
				if len(got) == len(args) {
					assert.Equal(t, args, got)
					assert.Contains(t, log.String(), "Failed to get cgroup CPU request, falling back to JVM defaults: ")
				} else {
					require.Len(t, got, len(args)+1)
					assert.Equal(t, args, got[:len(args)])
					assert.Regexp(t, `^-XX:ParallelGCThreads=[1-9][0-9]*$`, got[len(args)])
					assert.NotContains(t, log.String(), "Failed to get cgroup CPU request")
				}
				return
			}
			assert.NotContains(t, log.String(), "Failed to get cgroup CPU request")
			assert.Equal(t, args, got)
		})
	}
}

func TestEnsureGCThreads(t *testing.T) {
	t.Setenv("CONTAINER", "true")
	// A two-CPU request (2048 shares) becomes cpu.weight=79 in the container runtime.
	filesystem := fstest.MapFS{
		"proc/self/mountinfo":      &fstest.MapFile{Data: []byte("36 25 0:33 / /sys/fs/cgroup rw - cgroup2 cgroup rw\n")},
		"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte("79\n")},
	}
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "adds parallel threads preserving other options",
			args: []string{"-XX:+UseG1GC", "-Dfoo=bar", "-XX:ConcGCThreads=2"},
			want: []string{"-XX:+UseG1GC", "-Dfoo=bar", "-XX:ConcGCThreads=2", "-XX:ParallelGCThreads=13"},
		},
		{
			name: "no explicit collector is inert",
			args: []string{"-Dfoo=bar", "-XX:ConcGCThreads=2"},
			want: []string{"-Dfoo=bar", "-XX:ConcGCThreads=2"},
		},
		{
			name: "explicit G1 gets parallel threads",
			args: []string{"-XX:+UseG1GC"},
			want: []string{"-XX:+UseG1GC", "-XX:ParallelGCThreads=13"},
		},
		{
			name: "G1 disable passes through untouched",
			args: []string{"-XX:+UseG1GC", "-XX:-UseG1GC"},
			want: []string{"-XX:+UseG1GC", "-XX:-UseG1GC"},
		},
		{
			name: "G1 re-enabled by later flag gets parallel threads",
			args: []string{"-XX:-UseG1GC", "-XX:+UseG1GC"},
			want: []string{"-XX:-UseG1GC", "-XX:+UseG1GC", "-XX:ParallelGCThreads=13"},
		},
		{
			name: "disabled non-G1 collector does not prevent sizing",
			args: []string{"-XX:-UseZGC", "-XX:+UseG1GC"},
			want: []string{"-XX:-UseZGC", "-XX:+UseG1GC", "-XX:ParallelGCThreads=13"},
		},
		{
			name: "non-G1 collector disabled by later flag is inert",
			args: []string{"-XX:+UseZGC", "-XX:-UseZGC"},
			want: []string{"-XX:+UseZGC", "-XX:-UseZGC"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ensureGCThreads(tc.args, filesystem, 16, nil, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEnsureGCThreads_ActiveProcessorCount(t *testing.T) {
	for _, value := range []string{"2", "0", "-1", "invalid"} {
		t.Run(value, func(t *testing.T) {
			args := []string{"-XX:+UseG1GC", "-XX:ActiveProcessorCount=" + value, "-XX:ConcGCThreads=2"}
			for _, heapSize := range []*uint64{nil, new(uint64(8 << 20))} {
				var log bytes.Buffer
				got, err := ensureGCThreads(args, fstest.MapFS{}, 16, heapSize, &log)
				require.NoError(t, err)
				assert.Equal(t, args, got)
				assert.Empty(t, log.String(), "processor override must prevent reading cgroups and applying the heap cap")
			}
		})
	}
}

func TestEnsureGCThreads_HeapCap(t *testing.T) {
	filesystem := fstest.MapFS{
		"proc/self/mountinfo":      &fstest.MapFile{Data: []byte("36 25 0:33 / /sys/fs/cgroup rw - cgroup2 cgroup rw\n")},
		"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte("79\n")},
	}
	for _, tc := range []struct {
		name               string
		maxHeapSizeInBytes *uint64
		hostProcessors     int
		wantThreads        int
	}{
		{name: "no absolute heap uses CPU sizing", hostProcessors: 16, wantThreads: 13},
		{name: "large heap does not bind", maxHeapSizeInBytes: new(uint64(4 << 30)), hostProcessors: 16, wantThreads: 13},
		{name: "256 MiB heap caps threads at six", maxHeapSizeInBytes: new(uint64(256 << 20)), hostProcessors: 16, wantThreads: 6},
		{name: "tiny heap caps threads at two", maxHeapSizeInBytes: new(uint64(8 << 20)), hostProcessors: 16, wantThreads: 2},
		{name: "zero heap still applies the minimum cap", maxHeapSizeInBytes: new(uint64(0)), hostProcessors: 16, wantThreads: 2},
		{name: "heap cap never increases CPU threads", maxHeapSizeInBytes: new(uint64(8 << 20)), hostProcessors: 1, wantThreads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"-XX:+UseG1GC", "-XX:ConcGCThreads=2"}
			var log bytes.Buffer
			got, err := ensureGCThreads(args, filesystem, tc.hostProcessors, tc.maxHeapSizeInBytes, &log)
			require.NoError(t, err)
			assert.Equal(t, append(args, fmt.Sprintf("-XX:ParallelGCThreads=%d", tc.wantThreads)), got)
			if tc.wantThreads < computeParallelGCThreads(2, tc.hostProcessors) {
				assert.Contains(t, log.String(), "Capping ParallelGCThreads")
			} else {
				assert.Empty(t, log.String())
			}
		})
	}
}

func TestEnsureGCThreads_NonG1Collectors(t *testing.T) {
	for _, collector := range []string{
		"UseSerialGC", "UseParallelGC", "UseZGC", "UseShenandoahGC", "UseEpsilonGC",
		"UseConcMarkSweepGC", "UseParNewGC", "UseParallelOldGC",
	} {
		t.Run(collector, func(t *testing.T) {
			flag := "-XX:+" + collector
			for _, tc := range []struct {
				name string
				args []string
			}{
				{name: "selected", args: []string{flag}},
				{name: "selected after G1", args: []string{"-XX:+UseG1GC", flag}},
				{name: "selected before G1", args: []string{flag, "-XX:+UseG1GC"}},
				{name: "re-enabled", args: []string{flag, "-XX:-" + collector, flag}},
				{name: "explicit concurrent threads", args: []string{flag, "-XX:ConcGCThreads=14"}},
				{name: "explicit thread counts", args: []string{flag, "-XX:ParallelGCThreads=3", "-XX:ConcGCThreads=4"}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var log bytes.Buffer
					got, err := ensureGCThreads(tc.args, fstest.MapFS{}, 48, new(uint64(8<<20)), &log)
					require.NoError(t, err)
					assert.Equal(t, tc.args, got)
					assert.Empty(t, log.String(), "inert limiter must not attempt to read cgroups")
				})
			}
		})
	}
}

func TestEnsureGCThreads_CGroupFailureUsesJVMDefaults(t *testing.T) {
	mountInfo := []byte("36 25 0:33 / /sys/fs/cgroup rw - cgroup2 cgroup rw\n")
	for _, tc := range []struct {
		name       string
		args       []string
		filesystem fstest.MapFS
	}{
		{name: "missing mount information", args: []string{"-XX:+UseG1GC"}, filesystem: fstest.MapFS{}},
		{
			name: "missing CPU request",
			args: []string{"-XX:+UseG1GC", "-Dfoo=bar", "-XX:ConcGCThreads=100"},
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{Data: mountInfo},
			},
		},
		{
			name: "invalid CPU request",
			args: []string{"-XX:+UseG1GC", "-XX:ConcGCThreads=100"},
			filesystem: fstest.MapFS{
				"proc/self/mountinfo":      &fstest.MapFile{Data: mountInfo},
				"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte("invalid\n")},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var log bytes.Buffer
			got, err := ensureGCThreads(tc.args, tc.filesystem, 48, new(uint64(8<<20)), &log)
			require.NoError(t, err)
			assert.Equal(t, tc.args, got)
			assert.Contains(t, log.String(), "Failed to get cgroup CPU request, falling back to JVM defaults: ")
		})
	}
}

func TestComputeParallelGCThreads(t *testing.T) {
	for _, tc := range []struct {
		hostProcessors int
		wantThreads    []int // CPU requests start at 1; the final value holds for all larger requests.
	}{
		{hostProcessors: 1, wantThreads: []int{1}},
		{hostProcessors: 4, wantThreads: []int{4}},
		{hostProcessors: 8, wantThreads: []int{8}},
		{hostProcessors: 48, wantThreads: []int{8, 13, 18, 23, 28, 33}},
		{hostProcessors: 64, wantThreads: []int{8, 13, 18, 23, 28, 33, 38, 43}},
		{hostProcessors: 128, wantThreads: []int{8, 13, 18, 23, 28, 33, 38, 43, 48, 53, 58, 63, 68, 73, 78, 83}},
	} {
		t.Run(fmt.Sprintf("%d_host_cores", tc.hostProcessors), func(t *testing.T) {
			for requestedCores := 1; requestedCores <= tc.hostProcessors; requestedCores++ {
				want := tc.wantThreads[min(requestedCores, len(tc.wantThreads))-1]
				assert.Equal(t, want, computeParallelGCThreads(uint(requestedCores), tc.hostProcessors),
					"CPU request: %d cores", requestedCores)
			}
		})
	}
}

func TestEnsureGCThreads_PreservesExplicitThreads(t *testing.T) {
	args := []string{
		"-XX:+UseG1GC", "-XX:ParallelGCThreads=1", "-XX:ParallelGCThreads=3",
		"-XX:ConcGCThreads=4", "-XX:ConcGCThreads=3",
	}
	got, err := ensureGCThreads(args, fstest.MapFS{}, 1, new(uint64(8<<20)), io.Discard)
	require.NoError(t, err)
	assert.Equal(t, args, got)
}

func TestEnsureGCThreads_ConcGCThreadsLeftUntouched(t *testing.T) {
	filesystem := fstest.MapFS{
		"proc/self/mountinfo":      &fstest.MapFile{Data: []byte("36 25 0:33 / /sys/fs/cgroup rw - cgroup2 cgroup rw\n")},
		"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte("100\n")},
	}
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "concurrent exceeding computed parallel is preserved",
			args: []string{"-XX:+UseG1GC", "-XX:ConcGCThreads=14"},
			want: []string{"-XX:+UseG1GC", "-XX:ConcGCThreads=14", "-XX:ParallelGCThreads=13"},
		},
		{
			name: "explicit parallel and concurrent are both preserved",
			args: []string{"-XX:ParallelGCThreads=3", "-XX:ConcGCThreads=4"},
			want: []string{"-XX:ParallelGCThreads=3", "-XX:ConcGCThreads=4"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ensureGCThreads(tc.args, filesystem, 16, nil, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFilterHeapSizeArgsV2(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		cgroupMemory     uint64
		heapPercentage   *float64
		allowHeapShrink  bool
		minHeapFreeRatio *int
		maxHeapFreeRatio *int
		wantMaxHeapSize  *uint64
		wantContains     []string
		wantNotContains  []string
	}{
		{
			name:            "allowHeapShrink false sets both Xms and Xmx",
			args:            []string{"-Dfoo=bar"},
			cgroupMemory:    10 * BytesInMebibyte,
			heapPercentage:  new(10.0),
			allowHeapShrink: false,
			wantMaxHeapSize: new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-Dfoo=bar",
				fmt.Sprintf("-Xms%d", 1*BytesInMebibyte),
				fmt.Sprintf("-Xmx%d", 1*BytesInMebibyte),
			},
		},
		{
			name:            "allowHeapShrink true omits Xms and strips AlwaysPreTouch",
			args:            []string{"-Dfoo=bar", "-XX:+AlwaysPreTouch"},
			cgroupMemory:    10 * BytesInMebibyte,
			heapPercentage:  new(10.0),
			allowHeapShrink: true,
			wantMaxHeapSize: new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-Dfoo=bar",
				fmt.Sprintf("-Xmx%d", 1*BytesInMebibyte),
			},
			wantNotContains: []string{
				fmt.Sprintf("-Xms%d", 1*BytesInMebibyte),
				"-XX:+AlwaysPreTouch",
			},
		},
		{
			name:            "allowHeapShrink false preserves AlwaysPreTouch",
			args:            []string{"-Dfoo=bar", "-XX:+AlwaysPreTouch"},
			cgroupMemory:    10 * BytesInMebibyte,
			heapPercentage:  new(10.0),
			allowHeapShrink: false,
			wantMaxHeapSize: new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-Dfoo=bar",
				"-XX:+AlwaysPreTouch",
				fmt.Sprintf("-Xms%d", 1*BytesInMebibyte),
				fmt.Sprintf("-Xmx%d", 1*BytesInMebibyte),
			},
		},
		{
			name:           "maximum percentage alone has no calculated absolute heap",
			args:           []string{"-Xmx1g", "-XX:MaxRAMPercentage=50.0"},
			cgroupMemory:   10 * BytesInMebibyte,
			heapPercentage: new(10.0),
			wantContains:   []string{"-XX:MaxRAMPercentage=50.0"},
			wantNotContains: []string{
				"-Xmx1g",
				fmt.Sprintf("-Xmx%d", BytesInMebibyte),
			},
		},
		{
			name:           "initial percentage alone has no calculated absolute heap",
			args:           []string{"-Xms1g", "-XX:InitialRAMPercentage=50.0"},
			cgroupMemory:   10 * BytesInMebibyte,
			heapPercentage: new(10.0),
			wantContains:   []string{"-XX:InitialRAMPercentage=50.0"},
			wantNotContains: []string{
				"-Xms1g",
				fmt.Sprintf("-Xmx%d", BytesInMebibyte),
			},
		},
		{
			name:            "existing RAMPercentage overrides are preserved",
			args:            []string{"-Dfoo=bar", "-XX:InitialRAMPercentage=50.0", "-XX:MaxRAMPercentage=80.0"},
			cgroupMemory:    10 * BytesInMebibyte,
			heapPercentage:  new(10.0),
			allowHeapShrink: true,
			wantContains: []string{
				"-Dfoo=bar",
				"-XX:InitialRAMPercentage=50.0",
				"-XX:MaxRAMPercentage=80.0",
			},
			wantNotContains: []string{
				fmt.Sprintf("-Xms%d", 1*BytesInMebibyte),
				fmt.Sprintf("-Xmx%d", 1*BytesInMebibyte),
			},
		},
		{
			name:             "min and max heap free ratio are appended when set",
			args:             []string{"-Dfoo=bar"},
			cgroupMemory:     10 * BytesInMebibyte,
			heapPercentage:   new(10.0),
			allowHeapShrink:  true,
			minHeapFreeRatio: new(20),
			maxHeapFreeRatio: new(40),
			wantMaxHeapSize:  new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-Dfoo=bar",
				"-XX:MinHeapFreeRatio=20",
				"-XX:MaxHeapFreeRatio=40",
			},
		},
		{
			name:             "configured ratio overrides value present in jvmOpts",
			args:             []string{"-Dfoo=bar", "-XX:MaxHeapFreeRatio=90", "-XX:MinHeapFreeRatio=5"},
			cgroupMemory:     10 * BytesInMebibyte,
			heapPercentage:   new(10.0),
			minHeapFreeRatio: new(20),
			maxHeapFreeRatio: new(40),
			wantMaxHeapSize:  new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-XX:MinHeapFreeRatio=20",
				"-XX:MaxHeapFreeRatio=40",
			},
			wantNotContains: []string{
				"-XX:MaxHeapFreeRatio=90",
				"-XX:MinHeapFreeRatio=5",
			},
		},
		{
			name:             "configured min ratio preserves max ratio in jvmOpts",
			args:             []string{"-XX:MinHeapFreeRatio=5", "-XX:MaxHeapFreeRatio=90"},
			cgroupMemory:     10 * BytesInMebibyte,
			heapPercentage:   new(10.0),
			minHeapFreeRatio: new(20),
			wantMaxHeapSize:  new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-XX:MinHeapFreeRatio=20",
				"-XX:MaxHeapFreeRatio=90",
			},
			wantNotContains: []string{"-XX:MinHeapFreeRatio=5"},
		},
		{
			name:             "configured max ratio preserves min ratio in jvmOpts",
			args:             []string{"-XX:MinHeapFreeRatio=5", "-XX:MaxHeapFreeRatio=90"},
			cgroupMemory:     10 * BytesInMebibyte,
			heapPercentage:   new(10.0),
			maxHeapFreeRatio: new(40),
			wantMaxHeapSize:  new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-XX:MinHeapFreeRatio=5",
				"-XX:MaxHeapFreeRatio=40",
			},
			wantNotContains: []string{"-XX:MaxHeapFreeRatio=90"},
		},
		{
			name:            "ratio present in jvmOpts is preserved when option is unset",
			args:            []string{"-Dfoo=bar", "-XX:MaxHeapFreeRatio=90"},
			cgroupMemory:    10 * BytesInMebibyte,
			heapPercentage:  new(10.0),
			wantMaxHeapSize: new(uint64(BytesInMebibyte)),
			wantContains: []string{
				"-XX:MaxHeapFreeRatio=90",
			},
		},
		{
			name:             "heap free ratio appended alongside RAMPercentage overrides",
			args:             []string{"-XX:InitialRAMPercentage=50.0", "-XX:MaxRAMPercentage=80.0"},
			cgroupMemory:     10 * BytesInMebibyte,
			heapPercentage:   new(10.0),
			minHeapFreeRatio: new(30),
			maxHeapFreeRatio: new(60),
			wantContains: []string{
				"-XX:InitialRAMPercentage=50.0",
				"-XX:MaxRAMPercentage=80.0",
				"-XX:MinHeapFreeRatio=30",
				"-XX:MaxHeapFreeRatio=60",
			},
			wantNotContains: []string{
				fmt.Sprintf("-Xms%d", 1*BytesInMebibyte),
				fmt.Sprintf("-Xmx%d", 1*BytesInMebibyte),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, maxHeapSizeInBytes := filterHeapSizeArgsV2(tc.args, tc.heapPercentage, tc.cgroupMemory, heapSizeOptions{
				allowHeapShrink:  tc.allowHeapShrink,
				minHeapFreeRatio: tc.minHeapFreeRatio,
				maxHeapFreeRatio: tc.maxHeapFreeRatio,
			})
			assert.Equal(t, tc.wantMaxHeapSize, maxHeapSizeInBytes)
			for _, want := range tc.wantContains {
				assert.Contains(t, result, want)
			}
			for _, notWant := range tc.wantNotContains {
				assert.NotContains(t, result, notWant)
			}
		})
	}
}

func TestCompileCmdNativeExecutionMode(t *testing.T) {
	// Create a temporary executable file
	tmpFile, err := os.CreateTemp("", "my-service-native")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	require.NoError(t, tmpFile.Close())
	defer func() { _ = os.Remove(tmpPath) }()

	t.Setenv("CONTAINER", "1")

	staticCfg := StaticLauncherConfig{
		TypedConfig: TypedConfig{Type: "java"}, // value irrelevant for native mode branch
	}

	customCfg := CustomLauncherConfig{
		TypedConfig: TypedConfig{Type: "java"},
		Experimental: ExperimentalLauncherConfig{
			ExecutionMode:             ExecutionModeNative,
			NativeImageExecutablePath: tmpPath,
			NativeImageArguments:      []string{"-XX:MaximumHeapSizePercent=50"},
			LimitGCThreads:            true,
		},
	}

	cgroups := map[string]string{}
	createLogger := func() (io.WriteCloser, error) { return &NoopClosingWriter{io.Discard}, nil }

	cmd, err := compileCmdFromConfig(&staticCfg, &customCfg, &cgroups, createLogger)
	require.NoError(t, err)

	// Expect command path and arguments
	wantArgs := []string{
		tmpPath,
		"-XX:MaximumHeapSizePercent=50",
	}
	assert.Equal(t, tmpPath, cmd.Path)
	assert.Equal(t, wantArgs, cmd.Args)
}

func TestCompileCmdNativeExecutionModeHeapPercentage(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "nativeExeHP")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	require.NoError(t, tmpFile.Close())
	defer func() { _ = os.Remove(tmpPath) }()

	t.Setenv("CONTAINER", "1")

	heap := 60.1

	staticCfg := StaticLauncherConfig{
		TypedConfig: TypedConfig{Type: "java"},
	}

	customCfg := CustomLauncherConfig{
		TypedConfig:    TypedConfig{Type: "java"},
		HeapPercentage: &heap,
		Experimental: ExperimentalLauncherConfig{
			ExecutionMode:             ExecutionModeNative,
			NativeImageExecutablePath: tmpPath,
			NativeImageArguments:      []string{},
		},
	}

	cgroups := map[string]string{}
	createLogger := func() (io.WriteCloser, error) { return &NoopClosingWriter{io.Discard}, nil }

	cmd, err := compileCmdFromConfig(&staticCfg, &customCfg, &cgroups, createLogger)
	require.NoError(t, err)

	assert.Contains(t, cmd.Args, "-XX:MaximumHeapSizePercent=60")
}
