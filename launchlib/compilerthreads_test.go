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

func TestCompilerCount(t *testing.T) {
	// The JVM default for each processor count, from CompilationPolicy::initialize.
	for processors, want := range map[int]int{1: 2, 2: 2, 3: 2, 4: 3, 7: 3, 8: 4, 15: 4, 16: 12, 20: 12, 31: 12, 32: 15, 64: 18} {
		assert.Equal(t, want, compilerCount(processors), "processors %d", processors)
	}
}

func TestEnsureCompilerThreads(t *testing.T) {
	args := []string{"-XX:+UseG1GC"}
	for _, test := range []struct {
		name       string
		filesystem fstest.MapFS
		host       int
		want       []string
	}{
		{name: "v2 50m", filesystem: cpuWeightFS("10"), host: 20, want: append(args, "-XX:CICompilerCount=2")},
		{name: "v2 200m", filesystem: cpuWeightFS("29"), host: 20, want: append(args, "-XX:CICompilerCount=2")},
		{name: "v2 500m", filesystem: cpuWeightFS("59"), host: 20, want: append(args, "-XX:CICompilerCount=3")},
		{name: "v2 1 core", filesystem: cpuWeightFS("100"), host: 20, want: append(args, "-XX:CICompilerCount=4")},
		{name: "v2 2 cores matches the JVM default", filesystem: cpuWeightFS("174"), host: 20, want: args},
		{name: "v1 200m", filesystem: cpuSharesFS("204"), host: 20, want: append(args, "-XX:CICompilerCount=2")},
		{name: "v1 1 core", filesystem: cpuSharesFS("1024"), host: 20, want: append(args, "-XX:CICompilerCount=4")},
		{name: "small host matches the JVM default", filesystem: cpuWeightFS("10"), host: 2, want: args},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, ensureCompilerThreads(args, test.filesystem, test.host, io.Discard))
		})
	}

	t.Run("CPU quota caps available processors", func(t *testing.T) {
		filesystem := cpuWeightFS("100")
		filesystem["sys/fs/cgroup/cpu.max"] = &fstest.MapFile{Data: []byte("200000 100000\n")}
		assert.Equal(t, args, ensureCompilerThreads(args, filesystem, 20, io.Discard))
	})

	for _, explicit := range []string{"-XX:CICompilerCount=6", "-XX:-CICompilerCountPerCPU", "-XX:ActiveProcessorCount=4",
		"-XX:ReservedCodeCacheSize=64m", "-Xint"} {
		t.Run("explicit "+explicit, func(t *testing.T) {
			explicitArgs := []string{"-XX:+UseG1GC", explicit}
			assert.Equal(t, explicitArgs, ensureCompilerThreads(explicitArgs, cpuWeightFS("10"), 20, io.Discard))
		})
	}

	t.Run("cgroup read failure", func(t *testing.T) {
		var log bytes.Buffer
		assert.Equal(t, args, ensureCompilerThreads(args, fstest.MapFS{}, 20, &log))
		assert.Contains(t, log.String(), "using JVM default compiler threads")
	})
}

func TestLimitCompilerThreadsEnabled(t *testing.T) {
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
				t.Setenv(limitCompilerThreadsEnvVar, *test.env)
			} else {
				require.NoError(t, os.Unsetenv(limitCompilerThreadsEnvVar))
			}
			config := &CustomLauncherConfig{Experimental: ExperimentalLauncherConfig{LimitCompilerThreads: test.config}}
			enabled, err := limitCompilerThreadsEnabled(config)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.enabled, enabled)
		})
	}
}
