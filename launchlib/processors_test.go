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
	// cpu.weight range is 1-10000, while cpu.shares was 2-262144
	// conversion: weight = (shares * 10000) / 262144
	lowCPUWeightContent  = []byte("3814\n")  // ~100 shares
	highCPUWeightContent = []byte("38146\n") // ~1000 shares
	badCPUWeightContent  = []byte(``)
)

func TestProcessorCounter(t *testing.T) {
	for _, test := range []struct {
		name                   string
		filesystem             fs.FS
		expectedProcessorCount uint
		expectedError          error
	}{
		{
			name: "fails when unable to read cpu.weight",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV2MountInfoContent,
				},
			},
			expectedError: errors.New("unable to open cpu.weight at expected location"),
		},
		{
			name: "fails when unable to parse cpu.weight",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV2MountInfoContent,
				},
				"sys/fs/cgroup/cpu.weight": &fstest.MapFile{
					Data: badCPUWeightContent,
				},
			},
			expectedError: errors.New("unable to convert cpu.weight value to expected type"),
		},
		{
			name: "returns expected processor count when cpu.weight under 2 cores equivalent",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV2MountInfoContent,
				},
				"sys/fs/cgroup/cpu.weight": &fstest.MapFile{
					Data: lowCPUWeightContent,
				},
			},
			expectedProcessorCount: 2,
		},
		{
			name: "returns expected processor count when cpu.weight over 2 cores equivalent",
			filesystem: fstest.MapFS{
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV2MountInfoContent,
				},
				"sys/fs/cgroup/cpu.weight": &fstest.MapFile{
					Data: highCPUWeightContent,
				},
			},
			expectedProcessorCount: 10,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			counter := launchlib.NewCGroupProcessorCounter(test.filesystem)
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
