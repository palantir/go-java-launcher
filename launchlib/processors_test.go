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
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: MountInfoContent,
				},
			},
			expectedError: errors.New("unable to open cpu.shares at expected location"),
		},
		{
			name: "fails when unable to parse cpu.shares",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: MountInfoContent,
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
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: MountInfoContent,
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
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: MountInfoContent,
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
