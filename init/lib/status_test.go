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

func TestGetNotRunningCmds_NoConfiguration(t *testing.T) {
	notRunningCmds, err := GetNotRunningCmds()
	assert.Nil(t, notRunningCmds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get commands from static and custom configuration files")
}

func TestGetNotRunningCmds_OneConfiguredNoPidfile(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	notRunningCmds, err := GetNotRunningCmds()
	assert.Nil(t, notRunningCmds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine running processes")
}

func TestGetNotRunningCmds_OneConfiguredOnePidWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", 99999)

	notRunningCmds, err := GetNotRunningCmds()
	assert.Equal(t, 1, len(notRunningCmds))
	_, ok := notRunningCmds["primary"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmds_OneConfiguredOnePidWrittenOneRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())

	notRunningCmds, err := GetNotRunningCmds()
	assert.Empty(t, notRunningCmds)
	assert.NoError(t, err)
}

func TestGetNotRunningCmds_TwoConfiguredNoPidfile(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	notRunningCmds, err := GetNotRunningCmds()
	assert.Nil(t, notRunningCmds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine running processes")
}

func TestGetNotRunningCmds_TwoConfiguredOnePidWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", 99999)

	notRunningCmds, err := GetNotRunningCmds()
	assert.Equal(t, 2, len(notRunningCmds))
	_, ok := notRunningCmds["primary"]
	assert.True(t, ok)
	_, ok = notRunningCmds["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmds_TwoConfiguredOnePidWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())

	notRunningCmds, err := GetNotRunningCmds()
	assert.Equal(t, 1, len(notRunningCmds))
	_, ok := notRunningCmds["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmds_TwoConfiguredTwoPidsWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", 99998)
	writePidOrFail(t, "sidecar", 99999)

	notRunningCmds, err := GetNotRunningCmds()
	assert.Equal(t, 2, len(notRunningCmds))
	_, ok := notRunningCmds["primary"]
	assert.True(t, ok)
	_, ok = notRunningCmds["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmds_TwoConfiguredTwoPidsWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", 99999)

	notRunningCmds, err := GetNotRunningCmds()
	assert.Equal(t, 1, len(notRunningCmds))
	_, ok := notRunningCmds["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmds_TwoConfiguredTwoPidsWrittenTwoRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", cmd.Process.Pid)

	notRunningCmds, err := GetNotRunningCmds()
	assert.Empty(t, notRunningCmds)
	assert.NoError(t, err)
}

func TestGetRunningProcs_NoPidfile(t *testing.T) {
	runningProcs, err := GetRunningProcs()
	assert.Nil(t, runningProcs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read pidfile")
}

func TestGetRunningProcs_EmptyPidfile(t *testing.T) {
	setup(t)
	defer teardown(t)
	_, err := os.Create(pidfile)
	require.NoError(t, err)

	runningProcs, err := GetRunningProcs()
	assert.Nil(t, runningProcs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize pidfile")
}

func TestGetRunningProcs_InvalidPidfile(t *testing.T) {
	setup(t)
	defer teardown(t)
	require.NoError(t, ioutil.WriteFile(pidfile, []byte("bogus\ndata"), 0666))

	runningProcs, err := GetRunningProcs()
	assert.Nil(t, runningProcs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize pidfile")
}

func TestGetRunningProcs_OnePidWrittenZeroRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	notRunningPid := 99999
	writePidOrFail(t, "primary", notRunningPid)

	runningProcs, err := GetRunningProcs()
	assert.Empty(t, runningProcs)
	assert.NoError(t, err)
}

func TestGetRunningProcs_OnePidWrittenOneRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())

	runningProcs, err := GetRunningProcs()
	assert.Equal(t, 1, len(runningProcs))
	assert.Equal(t, os.Getpid(), runningProcs["primary"].Pid)
	assert.NoError(t, err)
}

func TestGetRunningProcs_TwoPidsWrittenZeroRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	notRunningPid := 99998
	otherNotRunningPid := 99999
	writePidOrFail(t, "primary", notRunningPid)
	writePidOrFail(t, "sidecar", otherNotRunningPid)

	runningProcs, err := GetRunningProcs()
	assert.Empty(t, runningProcs)
	assert.NoError(t, err)
}

func TestGetRunningProcs_TwoPidsWrittenOneRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	notRunningPid := 99999
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", notRunningPid)

	runningProcs, err := GetRunningProcs()
	assert.Equal(t, 1, len(runningProcs))
	assert.Equal(t, os.Getpid(), runningProcs["primary"].Pid)
	assert.NoError(t, err)
}

func TestGetRunningProcs_TwoPidsWrittenTwoRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", cmd.Process.Pid)

	runningProcs, err := GetRunningProcs()
	assert.Equal(t, 2, len(runningProcs))
	assert.Equal(t, os.Getpid(), runningProcs["primary"].Pid)
	assert.Equal(t, cmd.Process.Pid, runningProcs["sidecar"].Pid)
	assert.NoError(t, err)
}

func TestGetConfiguredCommands_NoConfiguration(t *testing.T) {
	setup(t)
	defer teardown(t)

	cmds, err := GetConfiguredCommands()
	assert.Nil(t, cmds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read static and custom configuration files")
}

func TestGetConfiguredCommands_OneConfigured(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)
	cmds, err := GetConfiguredCommands()
	assert.Equal(t, 1, len(cmds))
	_, ok := cmds["primary"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetConfiguredCommands_TwoConfigured(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	cmds, err := GetConfiguredCommands()
	assert.Equal(t, 2, len(cmds))
	_, ok := cmds["primary"]
	assert.True(t, ok)
	_, ok = cmds["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}
