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
		{name: "CPU request above 200m", filesystem: cpuWeightFS("30"), heap: small, log: "CPU request is above 200m"},
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
