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
			expected:                []string{"-Dfoo=bar", "-XX:MaximumHeapSizePercent=60"},
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
				t.Setenv("CONTAINER", "1")
			} else {
				t.Setenv("CONTAINER", "")
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
