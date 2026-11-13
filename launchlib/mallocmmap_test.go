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

const pinnedThreshold = "glibc.malloc.mmap_threshold=131072"

func cpuWeightFS(weight string) fstest.MapFS {
	return fstest.MapFS{
		"proc/self/mountinfo": &fstest.MapFile{
			Data: []byte("36 25 0:33 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw\n"),
		},
		"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte(weight + "\n")},
	}
}

func cpuSharesFS(shares string) fstest.MapFS {
	return fstest.MapFS{
		"proc/self/cgroup": &fstest.MapFile{Data: []byte("10:cpu,cpuacct:/abc\n")},
		"proc/self/mountinfo": &fstest.MapFile{
			Data: []byte("5137 5093 0:41 /abc /sys/fs/cgroup/cpu,cpuacct ro,nosuid,nodev,noexec,relatime master:23 - cgroup cgroup rw,cpu,cpuacct\n"),
		},
		"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{Data: []byte(shares + "\n")},
	}
}

func TestCPURequestCores(t *testing.T) {
	for _, test := range []struct {
		name       string
		filesystem fstest.MapFS
		want       float64
	}{
		{name: "v1 no request", filesystem: cpuSharesFS("2"), want: 2.0 / 1024},
		{name: "v1 200m", filesystem: cpuSharesFS("204"), want: 204.0 / 1024},
		{name: "v1 1.5 cores", filesystem: cpuSharesFS("1536"), want: 1.5},
		// cpu.weight values are what runc >= 1.3.2 and crun >= 1.23 set for each CPU request, and read back as the top
		// of the range of shares that map to them.
		{name: "v2 minimum weight", filesystem: cpuWeightFS("1"), want: 2.0 / 1024},
		{name: "v2 200m", filesystem: cpuWeightFS("29"), want: 207.11 / 1024},
		{name: "v2 1 core", filesystem: cpuWeightFS("100"), want: 1},
		{name: "v2 4 cores", filesystem: cpuWeightFS("303"), want: 4108.27 / 1024},
		{name: "v2 maximum weight", filesystem: cpuWeightFS("10000"), want: 256},
	} {
		t.Run(test.name, func(t *testing.T) {
			counter, err := newCGroupProcessorCounter(test.filesystem)
			require.NoError(t, err)
			got, err := counter.cpuRequestCores()
			require.NoError(t, err)
			assert.InDelta(t, test.want, got, 0.0001)
		})
	}

	for _, test := range []struct {
		name       string
		filesystem fstest.MapFS
	}{
		{name: "v2 weight 0", filesystem: cpuWeightFS("0")},
		{name: "v2 weight above maximum", filesystem: cpuWeightFS("10001")},
		{name: "v2 weight not a number", filesystem: cpuWeightFS("lots")},
		{name: "v1 shares 0", filesystem: cpuSharesFS("0")},
	} {
		t.Run(test.name, func(t *testing.T) {
			counter, err := newCGroupProcessorCounter(test.filesystem)
			require.NoError(t, err)
			_, err = counter.cpuRequestCores()
			assert.Error(t, err)
		})
	}
}

func TestEnsureMallocMmapThreshold(t *testing.T) {
	small := uint64(376 * BytesInMebibyte)
	require.NoError(t, os.Unsetenv(glibcTunablesEnvVar))

	t.Run("small service gets the threshold", func(t *testing.T) {
		got := ensureMallocMmapThreshold(map[string]string{"FOO": "bar"}, cpuWeightFS("10"), small, io.Discard)
		assert.Equal(t, map[string]string{"FOO": "bar", glibcTunablesEnvVar: pinnedThreshold}, got)
	})

	t.Run("1 GiB heap at 200m is included", func(t *testing.T) {
		got := ensureMallocMmapThreshold(map[string]string{}, cpuWeightFS("29"), 1024*BytesInMebibyte, io.Discard)
		assert.Equal(t, pinnedThreshold, got[glibcTunablesEnvVar])
	})

	for _, test := range []struct {
		name       string
		filesystem fstest.MapFS
		pinned     bool
	}{
		{name: "v2 50m", filesystem: cpuWeightFS("10"), pinned: true},
		{name: "v2 200m", filesystem: cpuWeightFS("29"), pinned: true},
		{name: "v2 210m", filesystem: cpuWeightFS("30"), pinned: false},
		{name: "v2 1 core", filesystem: cpuWeightFS("100"), pinned: false},
		{name: "v1 200m", filesystem: cpuSharesFS("204"), pinned: true},
		{name: "v1 205m", filesystem: cpuSharesFS("209"), pinned: true},
		{name: "v1 206m", filesystem: cpuSharesFS("210"), pinned: false},
	} {
		t.Run("CPU request boundary "+test.name, func(t *testing.T) {
			got := ensureMallocMmapThreshold(map[string]string{}, test.filesystem, small, io.Discard)
			_, pinned := got[glibcTunablesEnvVar]
			assert.Equal(t, test.pinned, pinned)
		})
	}

	t.Run("configured tunables are kept", func(t *testing.T) {
		env := map[string]string{glibcTunablesEnvVar: "glibc.malloc.arena_max=2"}
		got := ensureMallocMmapThreshold(env, cpuWeightFS("10"), small, io.Discard)
		assert.Equal(t, "glibc.malloc.arena_max=2:"+pinnedThreshold, got[glibcTunablesEnvVar])
		assert.Equal(t, "glibc.malloc.arena_max=2", env[glibcTunablesEnvVar], "input env must not be mutated")
	})

	t.Run("inherited tunables are kept", func(t *testing.T) {
		t.Setenv(glibcTunablesEnvVar, "glibc.malloc.tcache_count=0")
		got := ensureMallocMmapThreshold(map[string]string{}, cpuWeightFS("10"), small, io.Discard)
		assert.Equal(t, "glibc.malloc.tcache_count=0:"+pinnedThreshold, got[glibcTunablesEnvVar])
	})

	t.Run("existing threshold is left alone", func(t *testing.T) {
		env := map[string]string{glibcTunablesEnvVar: "glibc.malloc.mmap_threshold=1048576"}
		assert.Equal(t, env, ensureMallocMmapThreshold(env, cpuWeightFS("10"), small, io.Discard))
	})

	for _, test := range []struct {
		name       string
		filesystem fstest.MapFS
		heap       uint64
		log        string
	}{
		{name: "heap above 1 GiB", filesystem: cpuWeightFS("10"), heap: 1025 * BytesInMebibyte, log: "max heap"},
		{name: "CPU request above 200m", filesystem: cpuWeightFS("30"), heap: small, log: "CPU request of 0.211 cores is above 200m"},
		{name: "cgroup read failure", filesystem: fstest.MapFS{}, heap: small, log: "failed to get cgroup CPU request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var log bytes.Buffer
			env := map[string]string{"FOO": "bar"}
			assert.Equal(t, env, ensureMallocMmapThreshold(env, test.filesystem, test.heap, &log))
			assert.Contains(t, log.String(), test.log)
		})
	}
}

func TestPinMallocMmapThresholdEnabled(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  bool
		env     *string
		enabled bool
		wantErr bool
	}{
		{name: "disabled by default", enabled: false},
		{name: "enabled in config", config: true, enabled: true},
		{name: "env enables", env: stringPtr("true"), enabled: true},
		{name: "env disables config", config: true, env: stringPtr("false"), enabled: false},
		{name: "empty env is ignored", config: true, env: stringPtr(""), enabled: true},
		{name: "invalid env is an error", config: true, env: stringPtr("maybe"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.env != nil {
				t.Setenv(pinMallocMmapThresholdEnvVar, *test.env)
			} else {
				require.NoError(t, os.Unsetenv(pinMallocMmapThresholdEnvVar))
			}
			config := &CustomLauncherConfig{Experimental: ExperimentalLauncherConfig{PinMallocMmapThreshold: test.config}}
			enabled, err := pinMallocMmapThresholdEnabled(config)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.enabled, enabled)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
