// Copyright 2023 Palantir Technologies, Inc.
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

package launchlib_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/palantir/go-java-launcher/launchlib"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	memoryLimitContent    = []byte("2147483648\n")
	badMemoryLimitContent = []byte(``)
)

const MountInfoContentV1Memory = `28 21 0:25 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,memory`

const CGroupV1Content = `12:memory:/user.slice/user-1000.slice/session-2.scope
11:devices:/user.slice/user-1000.slice/session-2.scope
10:freezer:/
9:blkio:/user.slice
8:pids:/user.slice/user-1000.slice/session-2.scope
7:perf_event:/
6:cpu,cpuacct:/user.slice
5:hugetlb:/
4:cpuset:/
3:rdma:/
2:net_cls,net_prio:/
1:name=systemd:/user.slice/user-1000.slice/session-2.scope
0::/user.slice/user-1000.slice/session-2.scope`

const CGroupV2Content = `0::/user.slice/user-1000.slice/session-2.scope`

func TestMemoryLimit_DefaultMemoryLimit(t *testing.T) {
	for _, test := range []struct {
		name                string
		filesystem          fs.FS
		expectedMemoryLimit uint64
		expectedError       error
	}{
		{
			name: "fails when unable to read memory.limit_in_bytes",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: []byte(CGroupV1Content),
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(MountInfoContentV1Memory),
				},
			},
			expectedError: errors.New("unable to open memory.limit_in_bytes at expected location"),
		},
		{
			name: "fails when unable to parse memory.limit_in_bytes",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: []byte(CGroupV1Content),
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(MountInfoContentV1Memory),
				},
				"sys/fs/cgroup/memory/memory.limit_in_bytes": &fstest.MapFile{
					Data: badMemoryLimitContent,
				},
			},
			expectedError: errors.New("unable to convert memory.limit_in_bytes value to expected type"),
		},
		{
			name: "returns expected RAM percentage when memory.limit_in_bytes under 2 GiB",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: []byte(CGroupV1Content),
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(MountInfoContentV1Memory),
				},
				"sys/fs/cgroup/memory/memory.limit_in_bytes": &fstest.MapFile{
					Data: memoryLimitContent,
				},
			},
			expectedMemoryLimit: 1 << 31,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			limit := launchlib.NewCGroupV1MemoryLimit(test.filesystem)
			memoryLimit, err := limit.MemoryLimitInBytes()
			if test.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedMemoryLimit, memoryLimit)
		})
	}
}

func TestMemoryLimitV1(t *testing.T) {
	test := struct {
		name       string
		filesystem fstest.MapFS
	}{
		name: "memory limit v1",
		filesystem: fstest.MapFS{
			"proc/self/cgroup": &fstest.MapFile{
				Data: []byte(CGroupV1Content),
			},
			"proc/self/mountinfo": &fstest.MapFile{
				Data: []byte("28 21 0:25 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,memory"),
			},
			"sys/fs/cgroup/memory/memory.limit_in_bytes": &fstest.MapFile{
				Data: []byte("2147483648\n"),
			},
		},
	}

	limit := launchlib.NewMemoryLimit(test.filesystem)
	bytes, err := limit.MemoryLimitInBytes()
	require.NoError(t, err)
	assert.Equal(t, uint64(2147483648), bytes)
}

func TestMemoryLimitV2(t *testing.T) {
	test := struct {
		name       string
		filesystem fstest.MapFS
	}{
		name: "memory limit v2",
		filesystem: fstest.MapFS{
			"proc/self/cgroup": &fstest.MapFile{
				Data: []byte(CGroupV2Content),
			},
			"proc/self/mountinfo": &fstest.MapFile{
				Data: []byte("28 21 0:25 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:13 - cgroup2 cgroup2 rw"),
			},
			"sys/fs/cgroup/user.slice/user-1000.slice/session-2.scope/memory.max": &fstest.MapFile{
				Data: []byte("2147483648\n"),
			},
		},
	}

	limit := launchlib.NewMemoryLimit(test.filesystem)
	bytes, err := limit.MemoryLimitInBytes()
	require.NoError(t, err)
	assert.Equal(t, uint64(2147483648), bytes)
}

func TestMemoryLimitV2Unlimited(t *testing.T) {
	test := struct {
		name       string
		filesystem fstest.MapFS
	}{
		name: "memory limit v2 unlimited",
		filesystem: fstest.MapFS{
			"proc/self/cgroup": &fstest.MapFile{
				Data: []byte(CGroupV2Content),
			},
			"proc/self/mountinfo": &fstest.MapFile{
				Data: []byte("28 21 0:25 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:13 - cgroup2 cgroup2 rw"),
			},
			"sys/fs/cgroup/user.slice/user-1000.slice/session-2.scope/memory.max": &fstest.MapFile{
				Data: []byte("max\n"),
			},
		},
	}

	limit := launchlib.NewMemoryLimit(test.filesystem)
	_, err := limit.MemoryLimitInBytes()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no memory limit set (unlimited)")
}
