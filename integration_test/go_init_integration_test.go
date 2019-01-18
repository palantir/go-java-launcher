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

package integration_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cli2 "github.com/palantir/go-java-launcher/init/cli"
	time2 "github.com/palantir/go-java-launcher/init/cli/time"
	"github.com/palantir/go-java-launcher/launchlib"
)

const (
	launcherStaticFile = "service/bin/launcher-static.yml"
	launcherCustomFile = "var/conf/launcher-custom.yml"
	logDir             = "var/log"
	outputLogFile      = "startup.log"
	pidfolder          = "var/run"
	pidfileFormat      = pidfolder + "/%s.pid"
)

var staticSingle, _, _ = launchlib.GetConfigsFromFiles("testdata/launcher-static.yml", "testdata/launcher-custom.yml",
	ioutil.Discard)
var staticMulti, _, _ = launchlib.GetConfigsFromFiles("testdata/launcher-static-multiprocess.yml",
	"testdata/launcher-custom-multiprocess.yml", ioutil.Discard)
var multiProcessSubProcessName = func() string {
	for name := range staticMulti.SubProcesses {
		return name
	}
	panic("multiprocess config doesn't have subProcess listed")
}()

var singleProcessPrimaryName = staticSingle.ServiceName
var multiProcessPrimaryName = staticMulti.ServiceName
var primaryOutputFile = filepath.Join(logDir, outputLogFile)
var subProcessOutputFile = fmt.Sprintf(filepath.Join(logDir, "%s-%s"), multiProcessSubProcessName, outputLogFile)

var files = []string{launcherStaticFile, launcherCustomFile}

type servicePids map[string]int

func setupBadConfig(t *testing.T) {
	setup(t)
	require.NoError(t, os.Link("testdata/launcher-static-bad-java-home.yml", launcherStaticFile))
}

func setupSingleProcess(t *testing.T) {
	setup(t)
	require.NoError(t, os.Link("testdata/launcher-static.yml", launcherStaticFile))
	require.NoError(t, os.Link("testdata/launcher-custom.yml", launcherCustomFile))
}

func setupMultiProcess(t *testing.T) {
	setup(t)
	require.NoError(t, os.Link("testdata/launcher-static-multiprocess.yml", launcherStaticFile))
	require.NoError(t, os.Link("testdata/launcher-custom-multiprocess.yml", launcherCustomFile))
}

func setup(t *testing.T) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		_ = <-c
		teardown(t)
	}()
	for _, file := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(file), 0755))
	}
}

func teardown(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.RemoveAll(strings.Split(file, "/")[0]))
	}
}

func TestInitStart_TruncatesStartupLogFile(t *testing.T) {
	setup(t)
	defer teardown(t)

	stringThatShouldDisappear := "this should disappear from the log file after starting"

	require.NoError(t, os.MkdirAll(filepath.Dir(primaryOutputFile), 0755))
	require.NoError(t, ioutil.WriteFile(primaryOutputFile, []byte(stringThatShouldDisappear), 0644))
	result := runInit(t, "start")

	assert.NotContains(t, result.startupLog, stringThatShouldDisappear)
}

/*
 * Each test tests what happens given a prior state. Prior states for 'start' and 'status' are defined by a triple
 * denoting the number of the following items: (number of commands configured, number of pids written to pidfiles,
 * number of processes running). For (a, b, c), only a >= b >= c is a valid input.
 */

/*
 * In these tests, start should exit 1.
 */

// (0, 0, 0)
func TestInitStart_NoConfig(t *testing.T) {
	setup(t)
	defer teardown(t)

	result := runInit(t, "start")

	assert.Equal(t, 1, result.exitCode)
	assert.Contains(t, result.startupLog, "failed to determine service status to determine what commands to run")
	assert.Contains(t, result.stderr, "failed to determine service status to determine what commands to run")
}

// (0, 0, 0)
func TestInitStart_BadConfig(t *testing.T) {
	setupBadConfig(t)
	defer teardown(t)

	result := runInit(t, "start")

	assert.Equal(t, 1, result.exitCode)
	assert.Contains(t, result.startupLog, "failed to determine service status to determine what commands to run")
	assert.Contains(t, result.stderr, "failed to determine service status to determine what commands to run")
}

/*
 * In these tests, start should exit 0 and do nothing.
 */

// (1, 1, 1)
func TestInitStart_OneConfiguredOneWrittenOneRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, map[string]int{singleProcessPrimaryName: os.Getpid()})
	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	pids := readPids(t)
	require.Len(t, pids, 1)
	assert.Equal(t, os.Getpid(), readPids(t)[singleProcessPrimaryName])
	assert.Empty(t, result.stderr)
}

// (2, 2, 2)
func TestInitStart_TwoConfiguredTwoWrittenTwoRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid(), multiProcessSubProcessName: cmd.Process.Pid})
	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assert.Equal(t, os.Getpid(), pids[multiProcessPrimaryName])
	assert.Equal(t, cmd.Process.Pid, pids[multiProcessSubProcessName])
	assert.Empty(t, result.stderr)
}

/*
 * In these tests, start should actually start something.
 */

func TestInitStart_CreatesDirs(t *testing.T) {
	defer teardown(t)

	setup(t)
	require.NoError(t, os.Link("testdata/launcher-static-with-dirs.yml", launcherStaticFile))

	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	// Wait for JVM to start up and print output
	time.Sleep(time.Second)
	// Re-read startup log to get what JVM wrote after go-init executed
	startupLog := readStartupLog(t)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, result.stderr)
	dir, err := os.Stat("foo")
	assert.NoError(t, err)
	assert.True(t, dir.IsDir())
	dir, err = os.Stat("bar/baz")
	assert.NoError(t, err)
	assert.True(t, dir.IsDir())

	require.NoError(t, os.RemoveAll("foo"))
	require.NoError(t, os.RemoveAll("bar"))
	pids := readPids(t)
	require.Len(t, pids, 1)
	// grep for testdata since it will be on the classpath
	assert.Equal(t, pgrepSinglePid(t, "testdata", os.Getpid()), pids["primary"])
	proc, _ := os.FindProcess(pids["primary"])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (1, 0, 0)
func TestInitStart_OneConfiguredZeroWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	time.Sleep(time.Second)
	startupLog := readStartupLog(t)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, result.stderr)
	pids := readPids(t)
	require.Len(t, pids, 1)
	assert.Equal(t, pgrepSinglePid(t, "testdata", os.Getpid()), pids[singleProcessPrimaryName])

	proc, _ := os.FindProcess(pids[singleProcessPrimaryName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (1, 1, 0)
func TestInitStart_OneConfiguredOneWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, servicePids{singleProcessPrimaryName: 99999})
	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	time.Sleep(time.Second)
	startupLog := readStartupLog(t)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, result.stderr)
	pids := readPids(t)
	require.Len(t, pids, 1)
	assert.Equal(t, pgrepSinglePid(t, "testdata", os.Getpid()), pids[singleProcessPrimaryName])

	proc, _ := os.FindProcess(pids[singleProcessPrimaryName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (2, 0, 0)
func TestInitStart_TwoConfiguredZeroWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	time.Sleep(time.Second)
	startupLog := readStartupLog(t)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, result.stderr)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assertContainSameElements(t, pgrepMultiPids(t, "testdata", os.Getpid()),
		[]int{pids[multiProcessPrimaryName], pids[multiProcessSubProcessName]})

	proc, _ := os.FindProcess(pids[multiProcessPrimaryName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
	proc, _ = os.FindProcess(pids[multiProcessSubProcessName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (2, 1, 0)
func TestInitStart_TwoConfiguredOneWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: 99999})
	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	time.Sleep(time.Second)
	startupLog := readStartupLog(t)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, result.stderr)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assertContainSameElements(t, pgrepMultiPids(t, "testdata", os.Getpid()),
		[]int{pids[multiProcessPrimaryName], pids[multiProcessSubProcessName]})

	proc, _ := os.FindProcess(pids[multiProcessPrimaryName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
	proc, _ = os.FindProcess(pids[multiProcessSubProcessName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (2, 1, 1)
func TestInitStart_TwoConfiguredOneWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid()})
	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	time.Sleep(time.Second)
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, result.stderr)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assert.Equal(t, os.Getpid(), pids[multiProcessPrimaryName])
	assert.Equal(t, pgrepSinglePid(t, "testdata", os.Getpid()), pids[multiProcessSubProcessName])

	proc, _ := os.FindProcess(pids[multiProcessSubProcessName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (2, 2, 0)
func TestInitStart_TwoConfiguredTwoWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: 99998, multiProcessSubProcessName: 99999})
	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	time.Sleep(time.Second)
	startupLog := readStartupLog(t)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, result.stderr)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assertContainSameElements(t, pgrepMultiPids(t, "testdata", os.Getpid()),
		[]int{pids[multiProcessPrimaryName], pids[multiProcessSubProcessName]})

	proc, _ := os.FindProcess(pids[multiProcessPrimaryName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
	proc, _ = os.FindProcess(pids[multiProcessSubProcessName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (2, 2, 1)
func TestInitStart_TwoConfiguredTwoWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid(), multiProcessSubProcessName: 99999})
	result := runInit(t, "start")

	assert.Equal(t, 0, result.exitCode)
	time.Sleep(time.Second)
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, result.stderr)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assert.Equal(t, os.Getpid(), pids[multiProcessPrimaryName])
	assert.Equal(t, pgrepSinglePid(t, "testdata", os.Getpid()), pids[multiProcessSubProcessName])

	proc, _ := os.FindProcess(pids[multiProcessSubProcessName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

/*
 * Same states apply to status.
 */

func TestInitStatus_DoesNotTruncateStartupLogFile(t *testing.T) {
	setup(t)
	defer teardown(t)

	stringThatShouldRemain := "this should remain in the log file after running"

	require.NoError(t, os.MkdirAll(filepath.Dir(primaryOutputFile), 0755))
	require.NoError(t, ioutil.WriteFile(primaryOutputFile, []byte(stringThatShouldRemain), 0644))
	result := runInit(t, "status")

	assert.Contains(t, result.startupLog, stringThatShouldRemain)
}

// (0, 0, 0)
func TestInitStatus_NoConfig(t *testing.T) {
	setup(t)
	defer teardown(t)

	result := runInit(t, "status")

	assert.Equal(t, 4, result.exitCode)
	assert.Contains(t, result.stderr, "failed to determine service status")
	assert.Contains(t, result.startupLog, "failed to determine service status")
}

// (0, 0, 0)
func TestInitStatus_BadConfig(t *testing.T) {
	setupBadConfig(t)
	defer teardown(t)

	result := runInit(t, "status")

	assert.Equal(t, 4, result.exitCode)
	assert.Contains(t, result.stderr, "failed to determine service status")
	assert.Contains(t, result.startupLog, "failed to determine service status")
}

// (1, 0, 0)
func TestInitStatus_OneConfiguredZeroWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	result := runInit(t, "status")

	assert.Equal(t, 3, result.exitCode)
	assert.Contains(t, result.stderr, fmt.Sprintf("commands '[%s]' are not running", singleProcessPrimaryName))
	assert.Contains(t, result.startupLog, fmt.Sprintf("commands '[%s]' are not running", singleProcessPrimaryName))
}

// (1, 1, 0)
func TestInitStatus_OneConfiguredOneWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, servicePids{singleProcessPrimaryName: 99999})
	result := runInit(t, "status")

	assert.Equal(t, 1, result.exitCode)
	assert.Contains(t, result.stderr, fmt.Sprintf("commands '[%s]' are not running", singleProcessPrimaryName))
	assert.Contains(t, result.startupLog, fmt.Sprintf("commands '[%s]' are not running", singleProcessPrimaryName))
}

// (1, 1, 1)
func TestInitStatus_OneConfiguredOneWrittenOneRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, servicePids{singleProcessPrimaryName: os.Getpid()})
	result := runInit(t, "status")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
}

// (2, 0, 0)
func TestInitStatus_TwoConfiguredZeroWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	result := runInit(t, "status")

	assert.Equal(t, 3, result.exitCode)
	assert.Contains(t, result.stderr, "commands")
	assert.Contains(t, result.stderr, multiProcessPrimaryName)
	assert.Contains(t, result.stderr, multiProcessSubProcessName)
	assert.Contains(t, result.stderr, "are not running")
	assert.Contains(t, result.startupLog, "commands")
	assert.Contains(t, result.startupLog, multiProcessPrimaryName)
	assert.Contains(t, result.startupLog, multiProcessSubProcessName)
	assert.Contains(t, result.startupLog, "are not running")
}

// (2, 1, 0)
func TestInitStatus_TwoConfiguredOneWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: 99999})
	result := runInit(t, "status")

	assert.Equal(t, 1, result.exitCode)
	assert.Contains(t, result.stderr, "commands")
	assert.Contains(t, result.stderr, multiProcessPrimaryName)
	assert.Contains(t, result.stderr, multiProcessSubProcessName)
	assert.Contains(t, result.stderr, "are not running")
	assert.Contains(t, result.startupLog, "commands")
	assert.Contains(t, result.startupLog, multiProcessPrimaryName)
	assert.Contains(t, result.startupLog, multiProcessSubProcessName)
	assert.Contains(t, result.startupLog, "are not running")
}

// (2, 1, 1)
func TestInitStatus_TwoConfiguredOneWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid()})
	result := runInit(t, "status")

	assert.Equal(t, 1, result.exitCode)
	assert.Contains(t, result.stderr, fmt.Sprintf("commands '[%s]' are not running", multiProcessSubProcessName))
	assert.Contains(t, result.startupLog, fmt.Sprintf("commands '[%s]' are not running",
		multiProcessSubProcessName))
}

// (2, 2, 0)
func TestInitStatus_TwoConfiguredTwoWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid(), multiProcessSubProcessName: 99999})
	result := runInit(t, "status")

	assert.Equal(t, 1, result.exitCode)
	assert.Contains(t, result.stderr, "commands")
	assert.Contains(t, result.stderr, multiProcessPrimaryName)
	assert.Contains(t, result.stderr, multiProcessSubProcessName)
	assert.Contains(t, result.stderr, "are not running")
	assert.Contains(t, result.startupLog, "commands")
	assert.Contains(t, result.startupLog, multiProcessPrimaryName)
	assert.Contains(t, result.startupLog, multiProcessSubProcessName)
	assert.Contains(t, result.startupLog, "are not running")
}

// (2, 2, 1)
func TestInitStatus_TwoConfiguredTwoWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid(), multiProcessSubProcessName: 99999})
	result := runInit(t, "status")

	assert.Equal(t, 1, result.exitCode)
	assert.Contains(t, result.stderr, fmt.Sprintf("commands '[%s]' are not running", multiProcessSubProcessName))
	assert.Contains(t, result.startupLog, fmt.Sprintf("commands '[%s]' are not running",
		multiProcessSubProcessName))
}

// (2, 2, 2)
func TestInitStatus_TwoConfiguredTwoWrittenTwoRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid(), multiProcessSubProcessName: cmd.Process.Pid})
	result := runInit(t, "status")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
}

func TestInitStop_DoesNotTruncateStartupLogFile(t *testing.T) {
	setup(t)
	defer teardown(t)

	stringThatShouldRemain := "this should remain in the log file after running"

	require.NoError(t, os.MkdirAll(filepath.Dir(primaryOutputFile), 0755))
	require.NoError(t, ioutil.WriteFile(primaryOutputFile, []byte(stringThatShouldRemain), 0644))
	result := runInit(t, "stop")

	assert.Contains(t, result.startupLog, stringThatShouldRemain)
}

/*
 * Prior states for stop are defined by pairs of the following: (number of processes written to pidfiles, number of
 * processes running). As above, for (a, b), only a >= b is a valid input.
 */

/*
 * In these tests, stop should exit 0 and do nothing.
 */

// (0, 0)
func TestInitStop_ZeroWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	result := runInit(t, "stop")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Empty(t, readPids(t))
}

// (1, 0)
func TestInitStop_OneWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, servicePids{singleProcessPrimaryName: 99999})
	result := runInit(t, "stop")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Empty(t, readPids(t))
}

// (2, 0)
func TestInitStop_TwoWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: 99998, multiProcessSubProcessName: 99999})
	result := runInit(t, "stop")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Empty(t, readPids(t))
}

/*
 * In these tests, stop should actually stop something.
 */

// (1, 1)
func TestInitStop_Stoppable_OneWrittenOneRunning(t *testing.T) {
	defer teardown(t)
	setupSingleProcess(t)

	pid, killer := forkKillableSleep(t)
	defer killer()
	writePids(t, servicePids{singleProcessPrimaryName: pid})
	result := runInit(t, "stop")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Empty(t, readPids(t))
}

// (2, 1)
func TestInitStop_Stoppable_TwoWrittenOneRunning(t *testing.T) {
	defer teardown(t)
	setupMultiProcess(t)

	pid, killer := forkKillableSleep(t)
	defer killer()
	writePids(t, servicePids{multiProcessPrimaryName: pid})
	result := runInit(t, "stop")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Empty(t, readPids(t))
}

// (2, 2)
func TestInitStop_Stoppable_TwoWrittenTwoRunning(t *testing.T) {
	defer teardown(t)
	setupMultiProcess(t)

	pid1, killer1 := forkKillableSleep(t)
	defer killer1()
	pid2, killer2 := forkKillableSleep(t)
	defer killer2()

	writePids(t, servicePids{multiProcessPrimaryName: pid1, multiProcessSubProcessName: pid2})
	result := runInit(t, "stop")

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Empty(t, readPids(t))
}

// (1, 1)
func TestInitStop_Unstoppable_OneWrittenOneRunning(t *testing.T) {
	defer teardown(t)
	setupSingleProcess(t)

	pid, killer := forkUnkillableSleep(t)
	defer killer()
	writePids(t, servicePids{singleProcessPrimaryName: pid})

	result := runStopAssertTimesOut(t)

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Contains(t, result.startupLog, "processes")
	assert.Contains(t, result.startupLog, "did not stop within 240 seconds, so a SIGKILL was sent")
}

// (2, 1)
func TestInitStop_Unstoppable_TwoWrittenOneRunning(t *testing.T) {
	defer teardown(t)
	setupMultiProcess(t)

	pid, killer := forkUnkillableSleep(t)
	defer killer()
	writePids(t, servicePids{multiProcessPrimaryName: pid, multiProcessSubProcessName: 99999})
	result := runStopAssertTimesOut(t)

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Contains(t, result.startupLog, "processes")
	assert.Contains(t, result.startupLog, "did not stop within 240 seconds, so a SIGKILL was sent")
}

// (2, 2)
func TestInitStop_Unstoppable_TwoWrittenTwoRunning(t *testing.T) {
	defer teardown(t)
	setupMultiProcess(t)

	pid1, killer1 := forkUnkillableSleep(t)
	defer killer1()
	pid2, killer2 := forkUnkillableSleep(t)
	defer killer2()

	writePids(t, servicePids{multiProcessPrimaryName: pid1, multiProcessSubProcessName: pid2})
	result := runStopAssertTimesOut(t)

	assert.Equal(t, 0, result.exitCode)
	assert.Empty(t, result.stderr)
	assert.Contains(t, result.startupLog, "processes")
	assert.Contains(t, result.startupLog, "did not stop within 240 seconds, so a SIGKILL was sent")
}

func forkKillableSleep(t *testing.T) (pid int, killer func()) {
	return forkAndGetPid(t, exec.Command("testdata/stoppable.sh"), syscall.SIGTERM)
}

func forkUnkillableSleep(t *testing.T) (pid int, killer func()) {
	return forkAndGetPid(t, exec.Command("testdata/unstoppable.sh"), syscall.SIGKILL)
}

// The returned 'killer' waits for the process to end (after killing), and asserts that it was killed by the
// expectedSignal.
func forkAndGetPid(t *testing.T, command *exec.Cmd, expectedSignal syscall.Signal) (pid int, killer func()) {
	launched := make(chan *os.Process)
	reaperChan := make(chan *syscall.WaitStatus)
	go func() {
		var b bytes.Buffer
		command.Stdout = &b
		command.Stderr = &b
		// Allow the process to signal it's ready by closing fd 3
		pr, pw, err := os.Pipe()
		require.NoError(t, err)
		command.ExtraFiles = append(command.ExtraFiles, pw)
		// To avoid races, start the process in the same goroutine where we wait for it.
		require.NoError(t, command.Start())
		// Close it after start, since we gave it to the command (by passing it to command.ExtraFiles)
		require.NoError(t, pw.Close())

		// Then, send back the process as soon as it's READY.
		go func() {
			// Wait until EOF
			_, e := io.Copy(ioutil.Discard, pr)
			require.NoError(t, e)
			launched <- command.Process
		}()

		// Reap it!
		waitStatus := waitProcess(t, command)

		exitcode := waitStatus.ExitStatus()
		output := b.String()
		pid := command.Process.Pid
		t.Logf("Process %d exited with exit code %v and output: '%s'\n", pid, exitcode, output)
		reaperChan <- waitStatus
	}()
	process := <-launched
	return process.Pid, func() {
		t.Logf("Teardown: killing process %v", process.Pid)
		if err := process.Kill(); err != nil && !strings.Contains(err.Error(),
			"os: process already finished") {
			t.Fatalf("failed to kill forked process with error %s", err.Error())
		}
		// Wait for the reaper so we get the logs of its output and exit status!
		waitStatus := <-reaperChan
		// Check that it exited after receiving the expected signal
		require.Equal(t, expectedSignal, waitStatus.Signal(),
			"Child process %d exited with unexpected signal", process.Pid)
	}
}

func waitProcess(t *testing.T, command *exec.Cmd) (waitStatus *syscall.WaitStatus) {
	err := command.Wait()
	state := command.ProcessState
	if err != nil {
		// try to get the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			t.Logf("command.Wait() for pid %d yielded ExitError: %v\n", state.Pid(), err)
			ws := exitError.Sys().(syscall.WaitStatus)
			return &ws
		} else {
			// This will happen (in OSX) if `name` is not available in $PATH,
			// in this situation, exit code could not be get, and stderr will be
			// empty string very likely, so we use the default fail code, and format err
			// to string and set to stderr
			t.Logf("Could not get exit code for failed program: %v. Error: %v\n", command.Args, err)
			return nil
		}
	} else {
		ws := state.Sys().(syscall.WaitStatus)
		t.Logf("Process %d exited with WaitStatus: %v\n", state.Pid(), ws)
		return &ws
	}
}

type initResult struct {
	exitCode   int
	stderr     string
	startupLog string
}

func runInit(t *testing.T, args ...string) initResult {
	return <-runInitWithClock(t, time2.NewRealClock(), args...)
}

func runInitWithClock(t *testing.T, clock time2.Clock, args ...string) <-chan initResult {
	var errbuf bytes.Buffer
	cli2.Clock = clock
	app := cli2.App()
	app.Stderr = &errbuf

	out := make(chan initResult)
	go func() {
		// Empty string as placeholder for executable path as would be the case in real invocation
		exitCode := app.Run(append([]string{""}, args...))
		stderr := errbuf.String()
		out <- initResult{exitCode: exitCode, stderr: stderr, startupLog: readStartupLog(t)}
	}()
	return out
}

func readStartupLog(t *testing.T) string {
	startupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	return string(startupLogBytes)
}

func writePids(t *testing.T, pids servicePids) {
	require.NoError(t, os.MkdirAll(pidfolder, 0755))
	for name, pid := range pids {
		require.NoError(t, ioutil.WriteFile(fmt.Sprintf(pidfileFormat, name), []byte(strconv.Itoa(pid)), 0644))
	}
}

func readPids(t *testing.T) servicePids {
	pids := servicePids{}

	err := filepath.Walk(pidfolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == pidfolder {
			return nil
		}

		parts := strings.Split(filepath.Base(path), ".")
		require.Len(t, parts, 2, "invalid pidfile format, does not have only a name and extension")
		require.Equal(t, parts[1], "pid", "invalid pidfile format, does not end with .pid")

		pidBytes, err := ioutil.ReadFile(path)
		require.NoError(t, err, "failed to read pidfile %s", path)
		pid, err := strconv.Atoi(string(pidBytes))
		require.NoError(t, err, "pidfile '%s', did not contain an integer", path)

		pids[parts[0]] = pid
		return nil
	})

	if !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	return pids
}

func pgrepSinglePid(t *testing.T, key string, ppid int) int {
	return pgrepMultiPids(t, key, ppid)[0]
}

func pgrepMultiPids(t *testing.T, key string, ppid int) []int {
	pidBytes, err := exec.Command("pgrep", "-f", "-P", strconv.Itoa(ppid), key).Output()
	require.NoError(t, err)
	pidsStrings := strings.Split(string(pidBytes), "\n")
	pids := make([]int, len(pidsStrings)-1)
	for i, pidString := range pidsStrings[:len(pidsStrings)-1] {
		pids[i], err = strconv.Atoi(pidString)
		require.NoError(t, err)
	}
	return pids
}

func assertContainSameElements(t *testing.T, a []int, b []int) {
	sort.Ints(a)
	sort.Ints(b)
	assert.Equal(t, a, b)
}

// Returns the read value if it was supplied within the timeout, otherwise nil.
func readFromChannel(c <-chan initResult, timeout time.Duration) (result *initResult) {
	select {
	case <-time.After(timeout):
		return nil
	case result := <-c:
		return &result
	}
}

// Runs init 'stop' and asserts that it will time out after 240 seconds.
func runStopAssertTimesOut(t *testing.T) *initResult {
	clock := time2.NewFakeClock()
	initChan := runInitWithClock(t, clock, "stop")
	clock.BlockUntil(2) // wait for timer and ticker to attach
	clock.Advance(239 * time.Second)
	result := readFromChannel(initChan, 1*time.Second)
	require.Nil(t, result, "Expected `stop` to still wait after 239 seconds")

	clock.Advance(1 * time.Second)
	result2 := readFromChannel(initChan, 1*time.Second)
	require.NotNil(t, result2, "Expected `stop` to finish after 240 seconds")

	return result2
}
