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

package lib

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"fmt"
)

func TestIsRunning_Running(t *testing.T) {
	process, _ := os.FindProcess(os.Getpid())
	assert.True(t, isRunning(process))
}

func TestIsRunning_NotRunning(t *testing.T) {
	process, _ := os.FindProcess(99999)
	assert.False(t, isRunning(process))
}

func TestGetProcessStatus_Running(t *testing.T) {
	setup(t)
	defer teardown(t)

	writePid(t, os.Getpid())
	process, status, err := GetProcessStatus()

	assert.Equal(t, process.Pid, os.Getpid())
	assert.Equal(t, status, 0)
	assert.NoError(t, err)
}

func TestGetProcessStatus_NotRunningPidfileExists(t *testing.T) {
	setup(t)
	defer teardown(t)

	notRunningPid := 99999
	writePid(t, notRunningPid)
	process, status, err := GetProcessStatus()

	assert.Equal(t, process.Pid, notRunningPid)
	assert.Equal(t, status, 1)
	assert.EqualError(t, err, "pidfile exists but process is not running")
}

func TestGetProcessStatus_NotRunningPidfileDoesNotExist(t *testing.T) {
	setup(t)
	defer teardown(t)

	process, status, err := GetProcessStatus()

	assert.Empty(t, process)
	assert.Equal(t, status, 3)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to read pidfile: open %s: no such file or directory", Pidfile))
}
