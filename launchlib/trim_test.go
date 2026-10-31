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
	"bytes"
	"io"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const trimArg = "-XX:TrimNativeHeapInterval=5000"

var (
	trimCGroupV1Content   = []byte("10:cpu,cpuacct:/abc\n")
	trimCGroupV1MountInfo = []byte("5137 5093 0:41 /abc /sys/fs/cgroup/cpu,cpuacct ro,nosuid,nodev,noexec,relatime " +
		"master:23 - cgroup cgroup rw,cpu,cpuacct\n")
	trimCGroupV2MountInfo = []byte("36 25 0:33 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw\n")
)

func cgroupV2CPUWeightFS(weight string) fstest.MapFS {
	return fstest.MapFS{
		"proc/self/mountinfo":      &fstest.MapFile{Data: trimCGroupV2MountInfo},
		"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte(weight + "\n")},
		"opt/java/release":         &fstest.MapFile{Data: []byte("IMPLEMENTOR=\"Amazon.com Inc.\"\nJAVA_VERSION=\"25.0.4\"\n")},
		"opt/java11/release":       &fstest.MapFile{Data: []byte("JAVA_VERSION=\"11.0.28\"\n")},
		"opt/java8/release":        &fstest.MapFile{Data: []byte("JAVA_VERSION=\"1.8.0_462\"\n")},
	}
}

func TestCPUSharesToWeight(t *testing.T) {
	for shares, want := range map[uint64]uint64{
		2:      1,
		102:    17,
		256:    35,
		1024:   100,
		2048:   174,
		3072:   240,
		262144: 10000,
	} {
		assert.Equal(t, want, cpuSharesToWeight(shares), "shares %d", shares)
	}
}

func TestCGroupCPURequestAbove(t *testing.T) {
	for _, test := range []struct {
		name   string
		weight string
		above  bool
	}{
		{name: "250m", weight: "35", above: false},
		{name: "1 core", weight: "100", above: false},
		{name: "2 cores", weight: "174", above: false},
		{name: "just over 2 cores", weight: "175", above: true},
		{name: "3 cores", weight: "240", above: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			above, _, err := cgroupCPURequestAbove(cgroupV2CPUWeightFS(test.weight), 2000)
			require.NoError(t, err)
			assert.Equal(t, test.above, above)
		})
	}

	for _, test := range []struct {
		shares string
		above  bool
	}{
		{shares: "2048", above: false},
		{shares: "2049", above: true},
	} {
		t.Run("cgroup v1 shares "+test.shares, func(t *testing.T) {
			above, _, err := cgroupCPURequestAbove(fstest.MapFS{
				"proc/self/cgroup":             &fstest.MapFile{Data: trimCGroupV1Content},
				"proc/self/mountinfo":          &fstest.MapFile{Data: trimCGroupV1MountInfo},
				"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{Data: []byte(test.shares + "\n")},
			}, 2000)
			require.NoError(t, err)
			assert.Equal(t, test.above, above)
		})
	}

	for _, weight := range []string{"0", "10001", "lots"} {
		t.Run("invalid weight "+weight, func(t *testing.T) {
			_, _, err := cgroupCPURequestAbove(cgroupV2CPUWeightFS(weight), 2000)
			assert.Error(t, err)
		})
	}
}

func TestEnsureTrimNativeHeap(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		weight   string
		javaHome string
		wantTrim bool
	}{
		{name: "small service", args: []string{"-Xms402653184", "-Xmx402653184"}, weight: "35", javaHome: "/opt/java", wantTrim: true},
		{name: "2 cores and 8 GiB heap", args: []string{"-Xmx8g"}, weight: "174", javaHome: "/opt/java", wantTrim: true},
		{name: "more than 2 cores", args: []string{"-Xmx512m"}, weight: "240", javaHome: "/opt/java", wantTrim: false},
		{name: "heap above 8 GiB", args: []string{"-Xmx8193m"}, weight: "35", javaHome: "/opt/java", wantTrim: false},
		{name: "large service", args: []string{"-Xmx160g"}, weight: "5000", javaHome: "/opt/java", wantTrim: false},
		{name: "unknown heap size", args: []string{"-XX:MaxRAMPercentage=75.0"}, weight: "35", javaHome: "/opt/java", wantTrim: false},
		{name: "java 11 without ignore flag", args: []string{"-Xmx512m"}, weight: "35", javaHome: "/opt/java11", wantTrim: false},
		{name: "java 8 without ignore flag", args: []string{"-Xmx512m"}, weight: "35", javaHome: "/opt/java8", wantTrim: false},
		{name: "missing release file without ignore flag", args: []string{"-Xmx512m"}, weight: "35", javaHome: "/missing", wantTrim: false},
		{
			name:     "java 11 with ignore flag",
			args:     []string{"-XX:+IgnoreUnrecognizedVMOptions", "-Xmx512m"},
			weight:   "35",
			javaHome: "/opt/java11",
			wantTrim: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ensureTrimNativeHeap(test.args, cgroupV2CPUWeightFS(test.weight), test.javaHome, io.Discard)
			if test.wantTrim {
				assert.Equal(t, append(append([]string{}, test.args...), trimArg), got)
			} else {
				assert.Equal(t, test.args, got)
			}
		})
	}

	t.Run("explicit interval is preserved", func(t *testing.T) {
		args := []string{"-Xmx512m", "-XX:TrimNativeHeapInterval=0"}
		assert.Equal(t, args, ensureTrimNativeHeap(args, cgroupV2CPUWeightFS("35"), "/opt/java", io.Discard))
	})

	t.Run("cgroup read failure leaves args unchanged", func(t *testing.T) {
		var log bytes.Buffer
		args := []string{"-Xmx512m"}
		assert.Equal(t, args, ensureTrimNativeHeap(args, fstest.MapFS{}, "/opt/java", &log))
		assert.Contains(t, log.String(), "Not trimming native heap: failed to get cgroup CPU request")
	})
}

func TestTrimNativeHeapEnabled(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  bool
		env     *string
		enabled bool
	}{
		{name: "disabled by default", enabled: false},
		{name: "enabled in config", config: true, enabled: true},
		{name: "env enables", env: ptr("true"), enabled: true},
		{name: "env disables config", config: true, env: ptr("false"), enabled: false},
		{name: "invalid env falls back to config", config: true, env: ptr("maybe"), enabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.env != nil {
				t.Setenv(trimNativeHeapEnvVar, *test.env)
			} else {
				require.NoError(t, os.Unsetenv(trimNativeHeapEnvVar))
			}
			config := &CustomLauncherConfig{Experimental: ExperimentalLauncherConfig{TrimNativeHeap: test.config}}
			assert.Equal(t, test.enabled, trimNativeHeapEnabled(config, io.Discard))
		})
	}
}

func TestParseJVMMemorySize(t *testing.T) {
	for value, want := range map[string]uint64{
		"402653184": 402653184,
		"1024k":     1024 * 1024,
		"512m":      512 * BytesInMebibyte,
		"8G":        8 * 1024 * BytesInMebibyte,
	} {
		got, err := parseJVMMemorySize(value)
		require.NoError(t, err)
		assert.Equal(t, want, got, value)
	}
	_, err := parseJVMMemorySize("lots")
	assert.Error(t, err)
}

func ptr(s string) *string {
	return &s
}
