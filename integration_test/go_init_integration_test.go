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
	"gopkg.in/yaml.v2"

	cli2 "github.com/palantir/go-java-launcher/init/cli"
	time2 "github.com/palantir/go-java-launcher/init/cli/time"
	"github.com/palantir/go-java-launcher/launchlib"
)

const (
	launcherStaticFile = "service/bin/launcher-static.yml"
	launcherCustomFile = "var/conf/launcher-custom.yml"
	logDir             = "var/log"
	outputLogFile      = "startup.log"
	pidfile            = "var/run/pids.yml"
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
	_, _ = runInit("start")

	startupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.NotContains(t, startupLog, stringThatShouldDisappear)
}

/*
 * Each test tests what happens given a prior state. Prior states for 'start' and 'status' are defined by a triple
 * denoting the number of the following items: (number of commands configured, number of pids written to the pidfile,
 * number of processes running). For (a, b, c), only a >= b >= c is a valid input.
 */

/*
 * In these tests, start should exit 1.
 */

// (0, 0, 0)
func TestInitStart_NoConfig(t *testing.T) {
	setup(t)
	defer teardown(t)

	exitCode, stderr := runInit("start")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "failed to determine service status to determine what commands to run")
}

// (0, 0, 0)
func TestInitStart_BadConfig(t *testing.T) {
	setupBadConfig(t)
	defer teardown(t)

	exitCode, stderr := runInit("start")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "failed to determine service status to determine what commands to run")
}

/*
 * In these tests, start should exit 0 and do nothing.
 */

// (1, 1, 1)
func TestInitStart_OneConfiguredOneWrittenOneRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, map[string]int{singleProcessPrimaryName: os.Getpid()})
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	pids := readPids(t)
	require.Len(t, pids, 1)
	assert.Equal(t, os.Getpid(), readPids(t)[singleProcessPrimaryName])
	assert.Empty(t, stderr)
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
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assert.Equal(t, os.Getpid(), pids[multiProcessPrimaryName])
	assert.Equal(t, cmd.Process.Pid, pids[multiProcessSubProcessName])
	assert.Empty(t, stderr)
}

/*
 * In these tests, start should actually start something.
 */

func TestInitStart_CreatesDirs(t *testing.T) {
	defer teardown(t)

	setup(t)
	require.NoError(t, os.Link("testdata/launcher-static-with-dirs.yml", launcherStaticFile))

	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	startupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, stderr)
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
	assert.Equal(t, pgrepSinglePid(t, "testdata", os.Getpid()), pids[""])
	proc, _ := os.FindProcess(pids[""])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

// (1, 0, 0)
func TestInitStart_OneConfiguredZeroWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	startupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, stderr)
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
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second) // Wait for JVM to start and print output
	startupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, stderr)
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

	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	primaryStartupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	primaryStartupLog := string(primaryStartupLogBytes)
	assert.Contains(t, primaryStartupLog, "Using JAVA_HOME")
	assert.Contains(t, primaryStartupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
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
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	primaryStartupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	primaryStartupLog := string(primaryStartupLogBytes)
	assert.Contains(t, primaryStartupLog, "Using JAVA_HOME")
	assert.Contains(t, primaryStartupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
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
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
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
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	primaryStartupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	primaryStartupLog := string(primaryStartupLogBytes)
	assert.Contains(t, primaryStartupLog, "Using JAVA_HOME")
	assert.Contains(t, primaryStartupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
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
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	sidecarStartupLogBytes, err := ioutil.ReadFile(subProcessOutputFile)
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
	pids := readPids(t)
	require.Len(t, pids, 2)
	assert.Equal(t, os.Getpid(), pids[multiProcessPrimaryName])
	assert.Equal(t, pgrepSinglePid(t, "testdata", os.Getpid()), pids[multiProcessSubProcessName])

	proc, _ := os.FindProcess(pids[multiProcessSubProcessName])
	require.NoError(t, proc.Signal(syscall.SIGKILL))
}

/*
 * Same states appy to status.
 */

func TestInitStatus_DoesNotTruncateStartupLogFile(t *testing.T) {
	setup(t)
	defer teardown(t)

	stringThatShouldRemain := "this should remain in the log file after running"

	require.NoError(t, os.MkdirAll(filepath.Dir(primaryOutputFile), 0755))
	require.NoError(t, ioutil.WriteFile(primaryOutputFile, []byte(stringThatShouldRemain), 0644))
	_, _ = runInit("status")

	startupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, stringThatShouldRemain)
}

// (0, 0, 0)
func TestInitStatus_NoConfig(t *testing.T) {
	setup(t)
	defer teardown(t)

	exitCode, stderr := runInit("status")

	assert.Equal(t, 4, exitCode)
	assert.Contains(t, stderr, "failed to determine service status")
}

// (0, 0, 0)
func TestInitStatus_BadConfig(t *testing.T) {
	setupBadConfig(t)
	defer teardown(t)

	exitCode, stderr := runInit("status")

	assert.Equal(t, 4, exitCode)
	assert.Contains(t, stderr, "failed to determine service status")
}

// (1, 0, 0)
func TestInitStatus_OneConfiguredZeroWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	exitCode, stderr := runInit("status")

	assert.Equal(t, 3, exitCode)
	assert.Contains(t, stderr, fmt.Sprintf("commands '[%s]' are not running", singleProcessPrimaryName))
}

// (1, 1, 0)
func TestInitStatus_OneConfiguredOneWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, servicePids{singleProcessPrimaryName: 99999})
	exitCode, stderr := runInit("status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, fmt.Sprintf("commands '[%s]' are not running", singleProcessPrimaryName))
}

// (1, 1, 1)
func TestInitStatus_OneConfiguredOneWrittenOneRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, servicePids{singleProcessPrimaryName: os.Getpid()})
	exitCode, stderr := runInit("status")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
}

// (2, 0, 0)
func TestInitStatus_TwoConfiguredZeroWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	exitCode, stderr := runInit("status")

	assert.Equal(t, 3, exitCode)
	assert.Contains(t, stderr, "commands")
	assert.Contains(t, stderr, multiProcessPrimaryName)
	assert.Contains(t, stderr, multiProcessSubProcessName)
	assert.Contains(t, stderr, "are not running")
}

// (2, 1, 0)
func TestInitStatus_TwoConfiguredOneWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: 99999})
	exitCode, stderr := runInit("status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "commands")
	assert.Contains(t, stderr, multiProcessPrimaryName)
	assert.Contains(t, stderr, multiProcessSubProcessName)
	assert.Contains(t, stderr, "are not running")
}

// (2, 1, 1)
func TestInitStatus_TwoConfiguredOneWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid()})
	exitCode, stderr := runInit("status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, fmt.Sprintf("commands '[%s]' are not running", multiProcessSubProcessName))
}

// (2, 2, 0)
func TestInitStatus_TwoConfiguredTwoWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid(), multiProcessSubProcessName: 99999})
	exitCode, stderr := runInit("status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "commands")
	assert.Contains(t, stderr, multiProcessPrimaryName)
	assert.Contains(t, stderr, multiProcessSubProcessName)
	assert.Contains(t, stderr, "are not running")
}

// (2, 2, 1)
func TestInitStatus_TwoConfiguredTwoWrittenOneRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: os.Getpid(), multiProcessSubProcessName: 99999})
	exitCode, stderr := runInit("status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, fmt.Sprintf("commands '[%s]' are not running", multiProcessSubProcessName))
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
	exitCode, stderr := runInit("status")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
}

func TestInitStop_DoesNotTruncateStartupLogFile(t *testing.T) {
	setup(t)
	defer teardown(t)

	stringThatShouldRemain := "this should remain in the log file after running"

	require.NoError(t, os.MkdirAll(filepath.Dir(primaryOutputFile), 0755))
	require.NoError(t, ioutil.WriteFile(primaryOutputFile, []byte(stringThatShouldRemain), 0644))
	_, _ = runInit("stop")

	startupLogBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, stringThatShouldRemain)
}

/*
 * Prior states for stop are defined by pairs of the following: (number of processes written to the pidfile, number of
 * processes running). As above, for (a, b), only a >= b is a valid input.
 */

/*
 * In these tests, stop should exit 0 and do nothing.
 */

// (0, 0)
func TestInitStop_ZeroWrittenZeroRunning(t *testing.T) {
	setup(t)
	defer teardown(t)

	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, fmt.Sprintf("open %s: no such file or directory", pidfile))
}

// (1, 0)
func TestInitStop_OneWrittenZeroRunning(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePids(t, servicePids{singleProcessPrimaryName: 99999})
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, fmt.Sprintf("open %s: no such file or directory", pidfile))
}

// (2, 0)
func TestInitStop_TwoWrittenZeroRunning(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePids(t, servicePids{multiProcessPrimaryName: 99998, multiProcessSubProcessName: 99999})
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, fmt.Sprintf("open %s: no such file or directory", pidfile))
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
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, fmt.Sprintf("open %s: no such file or directory", pidfile))
}

// (2, 1)
func TestInitStop_Stoppable_TwoWrittenOneRunning(t *testing.T) {
	defer teardown(t)
	setupMultiProcess(t)

	pid, killer := forkKillableSleep(t)
	defer killer()
	writePids(t, servicePids{multiProcessPrimaryName: pid})
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, fmt.Sprintf("open %s: no such file or directory", pidfile))
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
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, fmt.Sprintf("open %s: no such file or directory", pidfile))
}

// (1, 1)
func TestInitStop_Unstoppable_OneWrittenOneRunning(t *testing.T) {
	defer teardown(t)
	setupSingleProcess(t)

	pid, killer := forkUnkillableSleep(t)
	defer killer()
	writePids(t, servicePids{singleProcessPrimaryName: pid})

	exitCode, stderr := runStopAssertTimesOut(t)

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	logBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	log := string(logBytes)
	assert.Contains(t, log, "processes")
	assert.Contains(t, log, "did not stop within 240 seconds, so a SIGKILL was sent")
}

// (2, 1)
func TestInitStop_Unstoppable_TwoWrittenOneRunning(t *testing.T) {
	defer teardown(t)
	setupMultiProcess(t)

	pid, killer := forkUnkillableSleep(t)
	defer killer()
	writePids(t, servicePids{multiProcessPrimaryName: pid, multiProcessSubProcessName: 99999})
	exitCode, stderr := runStopAssertTimesOut(t)

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	logBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	log := string(logBytes)
	assert.Contains(t, log, "processes")
	assert.Contains(t, log, "did not stop within 240 seconds, so a SIGKILL was sent")
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
	exitCode, stderr := runStopAssertTimesOut(t)

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	logBytes, err := ioutil.ReadFile(primaryOutputFile)
	require.NoError(t, err)
	log := string(logBytes)
	assert.Contains(t, log, "processes")
	assert.Contains(t, log, "did not stop within 240 seconds, so a SIGKILL was sent")
}

func forkKillableSleep(t *testing.T) (pid int, killer func()) {
	return forkAndGetPid(t, exec.Command("/bin/sleep", "10000"))
}

func forkUnkillableSleep(t *testing.T) (pid int, killer func()) {
	return forkAndGetPid(t, exec.Command("testdata/unstoppable.sh"))
}

func forkAndGetPid(t *testing.T, command *exec.Cmd) (pid int, killer func()) {
	command.Stderr = nil
	command.Stdout = nil
	command.Stdin = nil
	require.NoError(t, command.Start())
	// Reap it!
	go func() {
		_, err := command.Process.Wait()
		require.NoError(t, err)
	}()
	return command.Process.Pid, func() {
		if err := command.Process.Kill(); err != nil && !strings.Contains(err.Error(),
			"os: process already finished") {
			t.Fatalf("failed to kill forked process with error %s", err.Error())
		}
	}
}

type RunInitResult struct {
	exitStatus int
	stderr     string
}

func runInit(args ...string) (int, string) {
	initResult := <-runInitWithClock(time2.NewRealClock(), args...)
	return initResult.exitStatus, initResult.stderr
}

func runInitWithClock(clock time2.Clock, args ...string) <-chan RunInitResult {
	var errbuf bytes.Buffer
	cli2.Clock = clock
	app := cli2.App()
	app.Stderr = &errbuf

	out := make(chan RunInitResult)
	go func() {
		// Empty string as placeholder for executable path as would be the case in real invocation
		exitStatus := app.Run(append([]string{""}, args...))
		stderr := errbuf.String()
		out <- RunInitResult{exitStatus, stderr}
	}()
	return out
}

func writePids(t *testing.T, pids servicePids) {
	servicePids := servicePids{}
	for name, pid := range pids {
		servicePids[name] = pid
	}
	servicePidsBytes, err := yaml.Marshal(servicePids)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(pidfile), 0755))
	require.NoError(t, ioutil.WriteFile(pidfile, servicePidsBytes, 0644))
}

func readPids(t *testing.T) servicePids {
	pidfileBytes, err := ioutil.ReadFile(pidfile)
	require.NoError(t, err)
	pidfileExists := !os.IsNotExist(err)
	if err != nil && pidfileExists {
		require.Fail(t, "failed to read pidfile")
	} else if !pidfileExists {
		return servicePids{}
	}
	var servicePids servicePids
	require.NoError(t, yaml.Unmarshal(pidfileBytes, &servicePids))
	return servicePids
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
func readFromChannel(c <-chan RunInitResult, timeout time.Duration) (result *RunInitResult) {
	select {
	case <-time.After(timeout):
		return nil
	case result := <-c:
		return &result
	}
}

// Runs init 'stop' and asserts that it will time out after 240 seconds.
func runStopAssertTimesOut(t *testing.T) (int, string) {
	clock := time2.NewFakeClock()
	initChan := runInitWithClock(clock, "stop")
	clock.BlockUntil(2) // wait for timer and ticker to attach
	clock.Advance(239 * time.Second)
	result := readFromChannel(initChan, 1*time.Second)
	require.Nil(t, result, "Expected `stop` to still wait after 239 seconds")

	clock.Advance(1 * time.Second)
	result2 := readFromChannel(initChan, 1*time.Second)
	require.NotNil(t, result2, "Expected `stop` to finish after 240 seconds")

	return result2.exitStatus, result2.stderr
}
