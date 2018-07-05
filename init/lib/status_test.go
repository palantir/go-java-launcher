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
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetServiceStatus_RunningSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	// Note that this is in fact a different command than is configured to run. The old bash init.sh relied on the
	// command line from ps containing the classpath and thus the name of the Java main class to verify that the running
	// process was indeed the same as the one configured to run. There is no good way to ensure this in the
	// multi-process case.
	writePidOrFail(t, "primary", os.Getpid())
	info, status, err := GetServiceStatus()

	assert.Equal(t, 1, len(info.RunningProcs))
	assert.Equal(t, info.RunningProcs[0].Pid, os.Getpid())
	assert.Equal(t, 0, len(info.NotRunningCmds))
	assert.Equal(t, 0, status)
	assert.NoError(t, err)
}

func TestGetServiceStatus_RunningMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", cmd.Process.Pid)
	info, status, err := GetServiceStatus()
	runningPids := make([]int, len(info.RunningProcs))
	for i, proc := range info.RunningProcs {
		runningPids[i] = proc.Pid
	}

	assert.Equal(t, 2, len(info.RunningProcs))
	assert.Contains(t, runningPids, os.Getpid())
	assert.Contains(t, runningPids, cmd.Process.Pid)
	assert.Equal(t, 0, len(info.NotRunningCmds))
	assert.Equal(t, 0, status)
	assert.NoError(t, err)
}

func TestGetServiceStatus_PartiallyRunningPidfileExistsMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	notRunningPid := 99999
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", notRunningPid)
	info, status, err := GetServiceStatus()

	assert.Equal(t, 1, len(info.RunningProcs))
	assert.Equal(t, info.RunningProcs[0].Pid, os.Getpid())
	assert.Equal(t, 1, len(info.NotRunningCmds))
	assert.Equal(t, 1, status)
	assert.EqualError(t, err, "pidfile exists and can be read but at least one process is not running")
}

func TestGetServiceStatus_PartiallyRunningPidfileExistsIncompleteMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePidOrFail(t, "primary", os.Getpid())
	info, status, err := GetServiceStatus()

	assert.Equal(t, 1, len(info.RunningProcs))
	assert.Equal(t, info.RunningProcs[0].Pid, os.Getpid())
	assert.Equal(t, 1, len(info.NotRunningCmds))
	assert.Equal(t, 1, status)
	assert.EqualError(t, err, "pidfile exists and can be read but at least one process is not running")
}

func TestGetServiceStatus_NotRunningPidfileExistsSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	notRunningPid := 99999
	writePidOrFail(t, "primary", notRunningPid)
	info, status, err := GetServiceStatus()

	assert.Equal(t, 0, len(info.RunningProcs))
	assert.Equal(t, 1, len(info.NotRunningCmds))
	assert.Equal(t, 1, status)
	assert.EqualError(t, err, "pidfile exists and can be read but at least one process is not running")
}

func TestGetServiceStatus_NotRunningPidfileExistsMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	notRunningPid := 99998
	otherNotRunningPid := 99999
	writePidOrFail(t, "primary", notRunningPid)
	writePidOrFail(t, "sidecar", otherNotRunningPid)
	info, status, err := GetServiceStatus()

	assert.Equal(t, 0, len(info.RunningProcs))
	assert.Equal(t, 2, len(info.NotRunningCmds))
	assert.Equal(t, 1, status)
	assert.EqualError(t, err, "pidfile exists and can be read but at least one process is not running")
}

func TestGetServiceStatus_NotRunningPidfileExistsIncompleteMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	notRunningPid := 99998
	writePidOrFail(t, "primary", notRunningPid)
	info, status, err := GetServiceStatus()

	assert.Equal(t, 0, len(info.RunningProcs))
	assert.Equal(t, 2, len(info.NotRunningCmds))
	assert.Equal(t, 1, status)
	assert.EqualError(t, err, "pidfile exists and can be read but at least one process is not running")
}

func TestGetServiceStatus_NotRunningPidfileDoesNotExist(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	info, status, err := GetServiceStatus()

	assert.Equal(t, 0, len(info.RunningProcs))
	assert.Equal(t, 1, len(info.NotRunningCmds))
	assert.Equal(t, 3, status)
	require.Error(t, err, "expected error")
	assert.Contains(t, err.Error(), "failed to read pidfile")
}

func TestGetServiceStatus_NotRunningPidFileIsEmpty(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	_, err := os.Create(pidfile)
	require.NoError(t, err)
	info, status, err := GetServiceStatus()

	assert.Equal(t, 0, len(info.RunningProcs))
	assert.Equal(t, 1, len(info.NotRunningCmds))
	assert.Equal(t, 3, status)
	require.Error(t, err, "expected error")
	assert.Contains(t, err.Error(), "failed to deserialize pidfile")
}

func TestGetServiceStatus_NotRunningPidFileContainsInvalidTokens(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	require.NoError(t, ioutil.WriteFile(pidfile, []byte("bogus\ndata"), 0666))
	info, status, err := GetServiceStatus()

	assert.Equal(t, 0, len(info.RunningProcs))
	assert.Equal(t, 1, len(info.NotRunningCmds))
	assert.Equal(t, 3, status)
	require.Error(t, err, "expected error")
	assert.Contains(t, err.Error(), "failed to deserialize pidfile")
}

func TestGetServiceStatus_NotRunningPidfileDoesNotExistConfigIsBad(t *testing.T) {
	info, status, err := GetServiceStatus()

	assert.Nil(t, info)
	assert.Equal(t, 3, status)
	require.Error(t, err, "expected error")
	assert.Contains(t, err.Error(), "failed to get commands from static and custom configuration files")
}
