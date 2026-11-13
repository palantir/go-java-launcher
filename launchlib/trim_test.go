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

func cgroupV2CPUWeightFS(weight string) fstest.MapFS {
	return fstest.MapFS{
		"proc/self/mountinfo": &fstest.MapFile{
			Data: []byte("36 25 0:33 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw\n"),
		},
		"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte(weight + "\n")},
	}
}

func TestEnsureTrimNativeHeap(t *testing.T) {
	for _, test := range []struct {
		name     string
		heap     uint64
		weight   string
		wantTrim bool
	}{
		// cpu.weight values are what runc >= 1.3.2 and crun >= 1.23 set for each CPU request.
		{name: "quarter core and small heap", heap: 384 * BytesInMebibyte, weight: "35", wantTrim: true},
		{name: "4 cores and 8 GiB heap", heap: 8 * 1024 * BytesInMebibyte, weight: "303", wantTrim: true},
		{name: "5 cores", heap: 512 * BytesInMebibyte, weight: "363", wantTrim: false},
		{name: "heap above 8 GiB", heap: 8193 * BytesInMebibyte, weight: "35", wantTrim: false},
		{name: "large service", heap: 160 * 1024 * BytesInMebibyte, weight: "5000", wantTrim: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"-Xmx1", "-XX:+UseG1GC"}
			got := ensureTrimNativeHeap(args, cgroupV2CPUWeightFS(test.weight), test.heap, 5, io.Discard)
			if test.wantTrim {
				assert.Equal(t, append(append([]string{}, args...), trimArg), got)
			} else {
				assert.Equal(t, args, got)
			}
		})
	}

	t.Run("explicit interval is preserved", func(t *testing.T) {
		args := []string{"-XX:TrimNativeHeapInterval=0"}
		assert.Equal(t, args, ensureTrimNativeHeap(args, cgroupV2CPUWeightFS("35"), 512*BytesInMebibyte, 5, io.Discard))
	})

	t.Run("custom interval is converted to milliseconds", func(t *testing.T) {
		got := ensureTrimNativeHeap(nil, cgroupV2CPUWeightFS("35"), 512*BytesInMebibyte, 30, io.Discard)
		assert.Equal(t, []string{"-XX:TrimNativeHeapInterval=30000"}, got)
	})

	t.Run("max interval fits in the JVM flag", func(t *testing.T) {
		got := ensureTrimNativeHeap(nil, cgroupV2CPUWeightFS("35"), 512*BytesInMebibyte,
			trimNativeHeapMaxIntervalSeconds, io.Discard)
		assert.Equal(t, []string{"-XX:TrimNativeHeapInterval=4294967000"}, got)
	})

	t.Run("cgroup read failure leaves args unchanged", func(t *testing.T) {
		var log bytes.Buffer
		args := []string{"-XX:+UseG1GC"}
		assert.Equal(t, args, ensureTrimNativeHeap(args, fstest.MapFS{}, 512*BytesInMebibyte, 5, &log))
		assert.Contains(t, log.String(), "Not trimming native heap: failed to get cgroup CPU request")
	})
}

func TestTrimNativeHeapEnabled(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  bool
		env     *string
		enabled bool
		wantErr bool
	}{
		{name: "disabled by default", enabled: false},
		{name: "enabled in config", config: true, enabled: true},
		{name: "env enables", env: ptr("true"), enabled: true},
		{name: "env disables config", config: true, env: ptr("false"), enabled: false},
		{name: "empty env is ignored", config: true, env: ptr(""), enabled: true},
		{name: "invalid env is an error", config: true, env: ptr("maybe"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.env != nil {
				t.Setenv(trimNativeHeapEnvVar, *test.env)
			} else {
				require.NoError(t, os.Unsetenv(trimNativeHeapEnvVar))
			}
			config := &CustomLauncherConfig{Experimental: ExperimentalLauncherConfig{TrimNativeHeap: test.config}}
			enabled, err := trimNativeHeapEnabled(config)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.enabled, enabled)
		})
	}
}

func TestTrimNativeHeapIntervalSeconds(t *testing.T) {
	assert.Equal(t, 5, trimNativeHeapIntervalSeconds(&CustomLauncherConfig{}))
	config := &CustomLauncherConfig{Experimental: ExperimentalLauncherConfig{TrimNativeHeapIntervalSeconds: new(15)}}
	assert.Equal(t, 15, trimNativeHeapIntervalSeconds(config))
}

func ptr(s string) *string {
	return &s
}
