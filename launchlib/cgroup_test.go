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
	CGroupV2MountInfoContent = []byte(`36 35 0:31 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:12 - cgroup2 cgroup2 rw,nsdelegate,memory_recursiveprot`)

	CGroupV2ControllersContent = []byte(`cpuset cpu io memory hugetlb pids rdma`)
)

func TestCGroupPather(t *testing.T) {
	for _, test := range []struct {
		name          string
		filesystem    fs.FS
		cgroupName    string
		expectedPath  string
		expectedError error
	}{
		{
			name:          "fails when unable to read mountinfo",
			filesystem:    fstest.MapFS{},
			expectedError: errors.New("failed to open mountinfo file"),
		},
		{
			name: "fails when cgroup2 is not mounted",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: []byte(`36 35 0:31 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:12 - tmpfs tmpfs rw`),
				},
			},
			expectedError: errors.New("cgroup v2 unified hierarchy not mounted"),
		},
		{
			name: "fails when unable to read controllers file",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV2MountInfoContent,
				},
			},
			expectedError: errors.New("failed to read available controllers"),
		},
		{
			name: "fails when controller is not enabled",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV2MountInfoContent,
				},
				"sys/fs/cgroup/cgroup.controllers": &fstest.MapFile{
					Data: CGroupV2ControllersContent,
				},
			},
			cgroupName:    "network",
			expectedError: errors.New(`controller "network" not enabled in cgroup v2 hierarchy`),
		},
		{
			name: "returns expected path for known cgroup name",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV2MountInfoContent,
				},
				"sys/fs/cgroup/cgroup.controllers": &fstest.MapFile{
					Data: CGroupV2ControllersContent,
				},
			},
			cgroupName:   "cpu",
			expectedPath: "/sys/fs/cgroup",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pather := launchlib.NewCGroupV2Pather(test.filesystem)
			path, err := pather.Path(launchlib.CGroupName(test.cgroupName))
			if test.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedPath, path)
		})
	}
}
