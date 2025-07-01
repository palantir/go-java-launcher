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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNativeArgsFromJVMOpts(t *testing.T) {
	cases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "all allowed options",
			input:    []string{"-Djava.io.tmpdir=/tmp", "-Djna.tmpdir=/jna", "-Dsun.net.inetaddr.ttl=60"},
			expected: []string{"-Djava.io.tmpdir=/tmp", "-Djna.tmpdir=/jna", "-Dsun.net.inetaddr.ttl=60"},
		},
		{
			name:     "mixed allowed and not allowed",
			input:    []string{"-Djava.io.tmpdir=/tmp", "-Xmx2g", "-Djna.tmpdir=/jna"},
			expected: []string{"-Djava.io.tmpdir=/tmp", "-Djna.tmpdir=/jna"},
		},
		{
			name:     "none allowed",
			input:    []string{"-Xmx2g", "-Xms1g"},
			expected: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := getNativeArgsFromJVMOpts(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetNativeArgs(t *testing.T) {
	withEnv := func(t *testing.T, vars map[string]string) {
		oldVals := make(map[string]*string)
		for k, v := range vars {
			if old, existed := lookupEnv(k); existed {
				oldCopy := old
				oldVals[k] = &oldCopy
			} else {
				oldVals[k] = nil
			}
			_ = setenv(k, v)
		}
		t.Cleanup(func() {
			for k, old := range oldVals {
				if old != nil {
					_ = setenv(k, *old)
				} else {
					_ = unsetenv(k)
				}
			}
		})
	}

	heap60 := 60.0
	tests := []struct {
		name                    string
		envContainer            bool
		disableContainerSupport bool
		heapPercentage          *float64
		input                   []string
		expected                []string
	}{
		{
			name:                    "container mode enabled, removes -Xmx/-Xms, adds percent",
			envContainer:            true,
			disableContainerSupport: false,
			input:                   []string{"-Xmx2g", "-Xms1g", "-Dfoo=bar"},
			heapPercentage:          &heap60,
			expected:                []string{"-Dfoo=bar", "-XX:MaximumHeapSizePercent=60.00"},
		},
		{
			name:                    "container mode enabled, MaximumHeapSizePercent opt already present",
			envContainer:            true,
			disableContainerSupport: false,
			input:                   []string{"-Dfoo=bar", "-XX:MaximumHeapSizePercent=80"},
			heapPercentage:          &heap60,
			expected:                []string{"-Dfoo=bar", "-XX:MaximumHeapSizePercent=80"},
		},
		{
			name:                    "container mode disabled via env, removes percent args, keeps non-percent args",
			envContainer:            false, // disable container mode
			disableContainerSupport: true,
			input:                   []string{"-Dfoo=bar", "-XX:MaximumHeapSizePercent=80", "-Xmx2g"},
			expected:                []string{"-Dfoo=bar", "-Xmx2g"},
		},
		{
			name:                    "container mode disabled via config, removes percent args, keeps non-percent args",
			envContainer:            true,
			disableContainerSupport: true, // disable container mode
			input:                   []string{"-Dfoo=bar", "-XX:MaximumHeapSizePercent=80", "-Xmx2g"},
			expected:                []string{"-Dfoo=bar", "-Xmx2g"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup env
			if tc.envContainer {
				withEnv(t, map[string]string{"CONTAINER": "1"})
			} else {
				withEnv(t, map[string]string{"CONTAINER": ""})
			}

			cfg := &CustomLauncherConfig{
				DisableContainerSupport: tc.disableContainerSupport,
				HeapPercentage:          tc.heapPercentage,
			}
			result := getNativeArgs(tc.input, cfg)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

// --- helpers for env ---
func setenv(key, value string) error {
	return testSetEnv(key, value)
}
func unsetenv(key string) error {
	return testUnsetEnv(key)
}
func lookupEnv(key string) (string, bool) {
	return testLookupEnv(key)
}

// These are wrappers so we can mock os env in tests if needed.
var (
	testSetEnv    = func(key, value string) error { return setEnvReal(key, value) }
	testUnsetEnv  = func(key string) error { return unsetEnvReal(key) }
	testLookupEnv = func(key string) (string, bool) { return lookupEnvReal(key) }
)

func setEnvReal(key, value string) error {
	return os.Setenv(key, value)
}
func unsetEnvReal(key string) error {
	return os.Unsetenv(key)
}
func lookupEnvReal(key string) (string, bool) {
	return os.LookupEnv(key)
}
