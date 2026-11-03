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
	"fmt"
	"io/fs"
	"math"
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

func TestProcessorCounter_CGroupV1(t *testing.T) {
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
					Data: CGroupV1MountInfoContent,
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
					Data: CGroupV1MountInfoContent,
				},
				"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{
					Data: badCPUSharesContent,
				},
			},
			expectedError: errors.New("unable to convert cpu.shares value to expected type"),
		},
		{
			name: "rounds requests below one core up to one",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV1MountInfoContent,
				},
				"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{
					Data: lowCPUSharesContent,
				},
			},
			expectedProcessorCount: 1,
		},
		{
			name: "returns whole requested cores without host clamping",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: CGroupV1MountInfoContent,
				},
				"sys/fs/cgroup/cpu/cpu.shares": &fstest.MapFile{
					Data: highCPUSharesContent,
				},
			},
			expectedProcessorCount: 9,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			counter, err := launchlib.NewCGroupProcessorCounter(test.filesystem)
			require.NoError(t, err)
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

func TestProcessorCounter_CGroupV2(t *testing.T) {
	for _, tc := range []struct {
		name      string
		weight    string
		want      uint
		wantError string
	}{
		{name: "minimum weight requests at least one core", weight: "1\n", want: 1},
		{name: "default weight requests one core", weight: "100\n", want: 1},
		{name: "weight below two-core request stays below two cores", weight: "173\n", want: 1},
		{name: "two-core request survives runtime rounding", weight: "174\n", want: 2},
		{name: "fractional core request is truncated", weight: "208\n", want: 2},
		{name: "four-core request survives runtime rounding", weight: "303\n", want: 4},
		{name: "eight-core request survives runtime rounding", weight: "532\n", want: 8},
		{name: "weight below maximum stays below 256 cores", weight: "9999\n", want: 255},
		{name: "maximum weight is not capped to host cores", weight: "10000\n", want: 256},
		{name: "zero weight is rejected", weight: "0\n", wantError: "invalid cpu.weight value"},
		{name: "negative weight is rejected", weight: "-1\n", wantError: "invalid cpu.weight value"},
		{name: "invalid weight is rejected", weight: "10001\n", wantError: "invalid cpu.weight value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filesystem := fstest.MapFS{
				"proc/self/mountinfo":      &fstest.MapFile{Data: CGroupV2MountInfoContent},
				"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte(tc.weight)},
				"sys/fs/cgroup/cpu.max":    &fstest.MapFile{Data: []byte("100000 100000\n")},
			}
			counter, err := launchlib.NewCGroupProcessorCounter(filesystem)
			require.NoError(t, err)
			got, err := counter.ProcessorCount()
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestProcessorCounter_CGroupV2_WholeCoreBoundaries(t *testing.T) {
	for cores := uint(1); cores <= 256; cores++ {
		// Use the upstream runtime conversion to check every whole-core boundary.
		l := math.Log2(float64(cores * 1024))
		weight := min(10000, int(math.Ceil(math.Pow(10, (l*l+125*l)/612.0-7.0/34.0))))
		for _, tc := range []struct {
			weight int
			want   uint
		}{
			{weight: weight, want: cores},
			{weight: weight - 1, want: max(1, cores-1)},
		} {
			t.Run(fmt.Sprintf("%d_cores/weight_%d", cores, tc.weight), func(t *testing.T) {
				filesystem := fstest.MapFS{
					"proc/self/mountinfo":      &fstest.MapFile{Data: CGroupV2MountInfoContent},
					"sys/fs/cgroup/cpu.weight": &fstest.MapFile{Data: []byte(fmt.Sprintf("%d\n", tc.weight))},
				}
				counter, err := launchlib.NewCGroupProcessorCounter(filesystem)
				require.NoError(t, err)
				got, err := counter.ProcessorCount()
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}
	}
}
