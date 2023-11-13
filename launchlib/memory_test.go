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
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: MountInfoContent,
				},
			},
			expectedError: errors.New("unable to open memory.limit_in_bytes at expected location"),
		},
		{
			name: "fails when unable to parse memory.limit_in_bytes",
			filesystem: fstest.MapFS{
				"proc/self/cgroup": &fstest.MapFile{
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: MountInfoContent,
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
					Data: CGroupContent,
				},
				"proc/self/mountinfo": &fstest.MapFile{
					Data: MountInfoContent,
				},
				"sys/fs/cgroup/memory/memory.limit_in_bytes": &fstest.MapFile{
					Data: memoryLimitContent,
				},
			},
			expectedMemoryLimit: 1 << 31,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			limit := launchlib.NewCGroupMemoryLimit(test.filesystem)
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
