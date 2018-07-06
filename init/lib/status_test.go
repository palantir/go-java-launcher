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

func TestGetNotRunningCmdsByName_NoConfiguration(t *testing.T) {
	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Nil(t, notRunningCmdsByName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get commands from static and custom configuration files")
}

func TestGetNotRunningCmdsByName_OneConfiguredNoPidfile(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Nil(t, notRunningCmdsByName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine running processes")
}

func TestGetNotRunningCmdsByName_OneConfiguredOnePidWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", 99999)

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Equal(t, 1, len(notRunningCmdsByName))
	_, ok := notRunningCmdsByName["primary"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmdsByName_OneConfiguredOnePidWrittenOneRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Empty(t, notRunningCmdsByName)
	assert.NoError(t, err)
}

func TestGetNotRunningCmdsByName_TwoConfiguredNoPidfile(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Nil(t, notRunningCmdsByName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine running processes")
}

func TestGetNotRunningCmdsByName_TwoConfiguredOnePidWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", 99999)

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Equal(t, 2, len(notRunningCmdsByName))
	_, ok := notRunningCmdsByName["primary"]
	assert.True(t, ok)
	_, ok = notRunningCmdsByName["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmdsByName_TwoConfiguredOnePidWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Equal(t, 1, len(notRunningCmdsByName))
	_, ok := notRunningCmdsByName["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmdsByName_TwoConfiguredTwoPidsWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", 99998)
	writePidOrFail(t, "sidecar", 99999)

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Equal(t, 2, len(notRunningCmdsByName))
	_, ok := notRunningCmdsByName["primary"]
	assert.True(t, ok)
	_, ok = notRunningCmdsByName["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmdsByName_TwoConfiguredTwoPidsWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", 99999)

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Equal(t, 1, len(notRunningCmdsByName))
	_, ok := notRunningCmdsByName["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetNotRunningCmdsByName_TwoConfiguredTwoPidsWrittenTwoRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", cmd.Process.Pid)

	notRunningCmdsByName, err := GetNotRunningCmdsByName()
	assert.Empty(t, notRunningCmdsByName)
	assert.NoError(t, err)
}

func TestGetRunningProcsByName_NoPidfile(t *testing.T) {
	runningProcsByName, err := GetRunningProcsByName()
	assert.Nil(t, runningProcsByName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read pidfile")
}

func TestGetRunningProcsByName_EmptyPidfile(t *testing.T) {
	setup(t)
	defer teardown(t)
	_, err := os.Create(pidfile)
	require.NoError(t, err)

	runningProcsByName, err := GetRunningProcsByName()
	assert.Nil(t, runningProcsByName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize pidfile")
}

func TestGetRunningProcsByName_InvalidPidfile(t *testing.T) {
	setup(t)
	defer teardown(t)
	require.NoError(t, ioutil.WriteFile(pidfile, []byte("bogus\ndata"), 0666))

	runningProcsByName, err := GetRunningProcsByName()
	assert.Nil(t, runningProcsByName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize pidfile")
}

func TestGetRunningProcsByName_OnePidWrittenZeroRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	notRunningPid := 99999
	writePidOrFail(t, "primary", notRunningPid)

	runningProcsByName, err := GetRunningProcsByName()
	assert.Empty(t, runningProcsByName)
	assert.NoError(t, err)
}

func TestGetRunningProcsByName_OnePidWrittenOneRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	writePidOrFail(t, "primary", os.Getpid())

	runningProcsByName, err := GetRunningProcsByName()
	assert.Equal(t, 1, len(runningProcsByName))
	assert.Equal(t, os.Getpid(), runningProcsByName["primary"].Pid)
	assert.NoError(t, err)
}

func TestGetRunningProcsByName_TwoPidsWrittenZeroRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	notRunningPid := 99998
	otherNotRunningPid := 99999
	writePidOrFail(t, "primary", notRunningPid)
	writePidOrFail(t, "sidecar", otherNotRunningPid)

	runningProcsByName, err := GetRunningProcsByName()
	assert.Empty(t, runningProcsByName)
	assert.NoError(t, err)
}

func TestGetRunningProcsByName_TwoPidsWrittenOneRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	notRunningPid := 99999
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", notRunningPid)

	runningProcsByName, err := GetRunningProcsByName()
	assert.Equal(t, 1, len(runningProcsByName))
	assert.Equal(t, os.Getpid(), runningProcsByName["primary"].Pid)
	assert.NoError(t, err)
}

func TestGetRunningProcsByName_TwoPidsWrittenTwoRunning(t *testing.T) {
	setup(t)
	defer teardown(t)
	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePidOrFail(t, "primary", os.Getpid())
	writePidOrFail(t, "sidecar", cmd.Process.Pid)

	runningProcsByName, err := GetRunningProcsByName()
	assert.Equal(t, 2, len(runningProcsByName))
	assert.Equal(t, os.Getpid(), runningProcsByName["primary"].Pid)
	assert.Equal(t, cmd.Process.Pid, runningProcsByName["sidecar"].Pid)
	assert.NoError(t, err)
}

func TestGetConfiguredCommandsByName_NoConfiguration(t *testing.T) {
	setup(t)
	defer teardown(t)

	cmdsByName, err := GetConfiguredCommandsByName()
	assert.Nil(t, cmdsByName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read static and custom configuration files")
}

func TestGetConfiguredCommandsByName_OneConfigured(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)
	cmdsByName, err := GetConfiguredCommandsByName()
	assert.Equal(t, 1, len(cmdsByName))
	_, ok := cmdsByName["primary"]
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestGetConfiguredCommandsByName_TwoConfigured(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)
	cmdsByName, err := GetConfiguredCommandsByName()
	assert.Equal(t, 2, len(cmdsByName))
	_, ok := cmdsByName["primary"]
	assert.True(t, ok)
	_, ok = cmdsByName["sidecar"]
	assert.True(t, ok)
	assert.NoError(t, err)
}
