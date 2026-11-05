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
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cpuWeightFS(weight string) fstest.MapFS {
	return fstest.MapFS{
		"proc/self/mountinfo": &fstest.MapFile{
			Data: []byte("36 25 0:33 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw\n"),
		},
		"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte(weight + "\n")},
		"sys/fs/cgroup/cpu.max":    &fstest.MapFile{Data: []byte("max 100000\n")},
	}
}

func cpuSharesFS(shares string) fstest.MapFS {
	return fstest.MapFS{
		"proc/self/cgroup": &fstest.MapFile{Data: []byte("10:cpu,cpuacct:/abc\n")},
		"proc/self/mountinfo": &fstest.MapFile{
			Data: []byte("5137 5093 0:41 /abc /sys/fs/cgroup/cpu,cpuacct ro,nosuid,nodev,noexec,relatime master:23 - cgroup cgroup rw,cpu,cpuacct\n"),
		},
		"sys/fs/cgroup/cpu/cpu.shares":        &fstest.MapFile{Data: []byte(shares + "\n")},
		"sys/fs/cgroup/cpu/cpu.cfs_quota_us":  &fstest.MapFile{Data: []byte("-1\n")},
		"sys/fs/cgroup/cpu/cpu.cfs_period_us": &fstest.MapFile{Data: []byte("100000\n")},
	}
}

func TestCPUSharesToWeight(t *testing.T) {
	// Kubernetes maps 200m to 204 shares, 1 core to 1024 and 2 cores to 2048.
	for shares, want := range map[uint64]uint64{2: 1, 204: 29, 1024: 100, 2048: 174, 262144: 10000} {
		assert.Equal(t, want, cpuSharesToWeight(shares), "shares %d", shares)
	}
}

func TestCGroupCPURequestAbove(t *testing.T) {
	for _, test := range []struct {
		name       string
		filesystem fstest.MapFS
		above      bool
	}{
		{name: "v2 50m", filesystem: cpuWeightFS("10"), above: false},
		{name: "v2 200m", filesystem: cpuWeightFS("29"), above: false},
		{name: "v2 just over 200m", filesystem: cpuWeightFS("30"), above: true},
		{name: "v2 1 core", filesystem: cpuWeightFS("100"), above: true},
		{name: "v1 200m", filesystem: cpuSharesFS("204"), above: false},
		{name: "v1 just over 200m", filesystem: cpuSharesFS("205"), above: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := readCgroupCPURequest(test.filesystem)
			require.NoError(t, err)
			assert.Equal(t, test.above, request.above(200))
		})
	}

	for _, weight := range []string{"0", "10001", "lots"} {
		t.Run("invalid weight "+weight, func(t *testing.T) {
			_, err := readCgroupCPURequest(cpuWeightFS(weight))
			assert.Error(t, err)
		})
	}
}
