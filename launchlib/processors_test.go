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
	lowCPUSharesContent  = []byte("100\n")
	highCPUSharesContent = []byte("10000\n")
	badCPUSharesContent  = []byte(``)
)

const MountInfoContentV1CPU = `28 21 0:25 / /sys/fs/cgroup/cpu rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,cpu`
const MountInfoContentV2 = `28 21 0:25 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:13 - cgroup2 cgroup2 rw`

func TestProcessorCounter_DefaultCGroupV1ProcessorCounter(t *testing.T) {
	for _, test := range []struct {
		name                   string
		filesystem             fs.FS
		expectedProcessorCount uint
		expectedError          error
	}{
		{
			name: "fails when unable to read cpu.shares",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: []byte(CGroupV1Content),
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(MountInfoContentV1CPU),
				},
			},
			expectedError: errors.New("unable to open cpu.shares at expected location"),
		},
		{
			name: "fails when unable to parse cpu.shares",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: []byte(CGroupV1Content),
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(MountInfoContentV1CPU),
				},
				"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{
					Data: badCPUSharesContent,
				},
			},
			expectedError: errors.New("unable to convert cpu.shares value to expected type"),
		},
		{
			name: "returns expected processor count when cpu.shares under 2 cores",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: []byte(CGroupV1Content),
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(MountInfoContentV1CPU),
				},
				"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{
					Data: lowCPUSharesContent,
				},
			},
			expectedProcessorCount: 2,
		},
		{
			name: "returns expected processor count when cpu.shares over 2 cores",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: []byte(CGroupV1Content),
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(MountInfoContentV1CPU),
				},
				"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{
					Data: highCPUSharesContent,
				},
			},
			expectedProcessorCount: 9,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			counter := launchlib.NewCGroupV1ProcessorCounter(test.filesystem)
			processorCount, err := counter.ProcessorCount()
			if test.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedProcessorCount, processorCount)
		})
	}
}

func TestProcessorCountV1(t *testing.T) {
	test := struct {
		name       string
		filesystem fstest.MapFS
	}{
		name: "processor count v1",
		filesystem: fstest.MapFS{
			"proc/self/cgroup": &fstest.MapFile{
				Data: []byte(CGroupV1Content),
			},
			"proc/self/mountinfo": &fstest.MapFile{
				Data: []byte("28 21 0:25 / /sys/fs/cgroup/cpu rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,cpu"),
			},
			"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{
				Data: []byte("2048\n"),
			},
		},
	}

	counter := launchlib.NewProcessorCounter(test.filesystem)
	count, err := counter.ProcessorCount()
	require.NoError(t, err)
	assert.Equal(t, uint(2), count)
}

func TestProcessorCountV2(t *testing.T) {
	test := struct {
		name       string
		filesystem fstest.MapFS
	}{
		name: "processor count v2",
		filesystem: fstest.MapFS{
			"proc/self/cgroup": &fstest.MapFile{
				Data: []byte(CGroupV2Content),
			},
			"proc/self/mountinfo": &fstest.MapFile{
				Data: []byte("28 21 0:25 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:13 - cgroup2 cgroup2 rw"),
			},
			"sys/fs/cgroup/user.slice/user-1000.slice/session-2.scope/cpu.max": &fstest.MapFile{
				Data: []byte("200000 100000\n"),
			},
		},
	}

	counter := launchlib.NewProcessorCounter(test.filesystem)
	count, err := counter.ProcessorCount()
	require.NoError(t, err)
	assert.Equal(t, uint(2), count)
}

func TestProcessorCountV2Unlimited(t *testing.T) {
	test := struct {
		name       string
		filesystem fstest.MapFS
	}{
		name: "processor count v2 unlimited",
		filesystem: fstest.MapFS{
			"proc/self/cgroup": &fstest.MapFile{
				Data: []byte(CGroupV2Content),
			},
			"proc/self/mountinfo": &fstest.MapFile{
				Data: []byte("28 21 0:25 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:13 - cgroup2 cgroup2 rw"),
			},
			"sys/fs/cgroup/user.slice/user-1000.slice/session-2.scope/cpu.max": &fstest.MapFile{
				Data: []byte("max 100000\n"),
			},
		},
	}

	counter := launchlib.NewProcessorCounter(test.filesystem)
	_, err := counter.ProcessorCount()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no CPU quota set")
}
