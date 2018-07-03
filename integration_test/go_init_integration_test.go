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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/palantir/godel/pkg/products/v2/products"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/palantir/go-java-launcher/init/lib"
	"github.com/palantir/go-java-launcher/launchlib"
)

var launcherStaticFile = "service/bin/launcher-static.yml"
var launcherCustomFile = "var/conf/launcher-custom.yml"
var pidfile = "var/run/pids.yaml"

var files = []string{launcherStaticFile, launcherCustomFile, "var/log/startup.log", pidfile}

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
	for _, file := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(file), 0777))
	}
}

func teardown(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.RemoveAll(strings.Split(file, "/")[0]))
	}
}

func writePid(t *testing.T, name string, pid int) {
	var servicePids lib.ServicePids
	if pidfileExists() {
		pidfileBytes, err := ioutil.ReadFile(pidfile)
		require.NoError(t, err)
		require.NoError(t, yaml.Unmarshal(pidfileBytes, &servicePids))
	} else {
		servicePids.PidsByName = make(map[string]int)
	}
	servicePids.PidsByName[name] = pid
	servicePidsBytes, err := yaml.Marshal(servicePids)
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(pidfile, servicePidsBytes, 0666))
}

func readPids(t *testing.T) *lib.ServicePids {
	pidfileBytes, err := ioutil.ReadFile(pidfile)
	require.NoError(t, err)
	var servicePids lib.ServicePids
	require.NoError(t, yaml.Unmarshal(pidfileBytes, &servicePids))
	return &servicePids
}

func pidfileExists() bool {
	if _, err := os.Stat(pidfile); err != nil {
		// The only piece of information from the error we care about is if the file exists.
		return !os.IsNotExist(err)
	} else {
		return true
	}
}

func TestInitStart_DoesNotRestartRunningSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePid(t, "primary", os.Getpid())
	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, os.Getpid(), readPids(t).PidsByName["primary"])
	assert.Empty(t, stderr)
}

func TestInitStart_DoesNotRestartRunningMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePid(t, "primary", os.Getpid())
	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePid(t, "sidecar", cmd.Process.Pid)
	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 0, exitCode)
	pids := readPids(t).PidsByName
	assert.Equal(t, os.Getpid(), pids["primary"])
	assert.Equal(t, cmd.Process.Pid, pids["sidecar"])
	assert.Empty(t, stderr)
}

func TestInitStart_StartsNotRunningPidfileExistsSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePid(t, "primary", 99999)
	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second) // Wait for JVM to start and print output
	startupLogBytes, err := ioutil.ReadFile(launchlib.PrimaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, stderr)
}

func TestInitStart_StartsNotRunningPidfileExistsMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePid(t, "primary", 99998)
	writePid(t, "sidecar", 99999)
	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	primaryStartupLogBytes, err := ioutil.ReadFile(launchlib.PrimaryOutputFile)
	require.NoError(t, err)
	primaryStartupLog := string(primaryStartupLogBytes)
	assert.Contains(t, primaryStartupLog, "Using JAVA_HOME")
	assert.Contains(t, primaryStartupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(fmt.Sprintf(launchlib.OutputFileFormat, "sidecar-"))
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
}

func TestInitStart_StartsPartiallyRunningPidfileExistsMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePid(t, "primary", os.Getpid())
	writePid(t, "sidecar", 99999)
	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	pids := readPids(t).PidsByName
	assert.Equal(t, os.Getpid(), pids["primary"])
	sidecarStartupLogBytes, err := ioutil.ReadFile(fmt.Sprintf(launchlib.OutputFileFormat, "sidecar-"))
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
}

func TestInitStart_StartsNotRunningPidfileDoesNotExist(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	startupLogBytes, err := ioutil.ReadFile(launchlib.PrimaryOutputFile)
	require.NoError(t, err)
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, stderr)
}

func TestInitStart_DoesNotStartNotRunningPidfileDoesNotExistConfigIsBad(t *testing.T) {
	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "failed to read static and custom configuration files")
}

func TestInitStart_StartsNotRunningPidfileDoesNotExistMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	exitCode, stderr := runInit(t, "start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second)
	primaryStartupLogBytes, err := ioutil.ReadFile(launchlib.PrimaryOutputFile)
	require.NoError(t, err)
	primaryStartupLog := string(primaryStartupLogBytes)
	assert.Contains(t, primaryStartupLog, "Using JAVA_HOME")
	assert.Contains(t, primaryStartupLog, "main method")
	sidecarStartupLogBytes, err := ioutil.ReadFile(fmt.Sprintf(launchlib.OutputFileFormat, "sidecar-"))
	require.NoError(t, err)
	sidecarStartupLog := string(sidecarStartupLogBytes)
	assert.Contains(t, sidecarStartupLog, "Using JAVA_HOME")
	assert.Contains(t, sidecarStartupLog, "main method")
	assert.Empty(t, stderr)
}

func TestInitStatus_RunningSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePid(t, "primary", os.Getpid())
	exitCode, stderr := runInit(t, "status")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
}

func TestInitStatus_RunningMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePid(t, "primary", os.Getpid())
	cmd := exec.Command("/bin/sleep", "10")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Signal(syscall.SIGKILL))
	}()
	writePid(t, "sidecar", cmd.Process.Pid)
	exitCode, stderr := runInit(t, "status")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
}

func TestInitStatus_NotRunningPidfileExistsSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePid(t, "primary", 99999)
	exitCode, stderr := runInit(t, "status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "pidfile exists and can be read but at least one process is not running")
}

func TestInitStatus_PartiallyRunningPidfileExistsMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePid(t, "primary", os.Getpid())
	writePid(t, "sidecar", 99999)
	exitCode, stderr := runInit(t, "status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "pidfile exists and can be read but at least one process is not running")
}

func TestInitStatus_NotRunningPidfileExistsMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePid(t, "primary", 99999)
	writePid(t, "sidecar", 99998)
	exitCode, stderr := runInit(t, "status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "pidfile exists and can be read but at least one process is not running")
}

func TestInitStatus_NotRunningPidfileDoesNotExistSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	exitCode, stderr := runInit(t, "status")

	assert.Equal(t, 3, exitCode)
	assert.Contains(t, stderr, "failed to read pidfile: open var/run/pids.yaml: no such file or directory")
}

func TestInitStatus_NotRunningPidfileDoesNotExistMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	exitCode, stderr := runInit(t, "status")

	assert.Equal(t, 3, exitCode)
	assert.Contains(t, stderr, "failed to read pidfile: open var/run/pids.yaml: no such file or directory")
}

func TestInitStop_StopsRunningAndFailsRunningDoesNotTerminate(t *testing.T) {
	defer teardown(t)

	/* 1) Stoppable single-process service stops. */
	setupSingleProcess(t)

	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	pidBytes, err := exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)
	writePid(t, "primary", pid)
	exitCode, stderr := runInit(t, "stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err = ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, "open var/run/pids.yaml: no such file or directory")

	// Reset since this is really two tests we have to run sequentially.
	teardown(t)

	/* 2) Unstoppable single-process service does not stop. */
	setupSingleProcess(t)

	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 10000 &").Run())
	pidBytes, err = exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pid, err = strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)
	writePid(t, "primary", pid)
	exitCode, stderr = runInit(t, "stop")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, fmt.Sprintf("failed to stop at least one process: failed to wait for all processes to "+
		"stop: processes with pids '[%d]' did not stop within 240 seconds", pid))

	pids := readPids(t).PidsByName
	process, _ := os.FindProcess(pids["primary"])
	require.NoError(t, process.Signal(syscall.SIGKILL))

	teardown(t)

	/* 3) Stoppable multi-process service stops. */
	setupMultiProcess(t)

	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	pidsBytes, err := exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pidsStrings := strings.Split(string(pidsBytes), "\n")
	sleepPids := make([]int, len(pidsStrings)-1)
	for i, pidString := range pidsStrings[0 : len(pidsStrings)-1] {
		sleepPids[i], err = strconv.Atoi(pidString)
		require.NoError(t, err)
	}
	writePid(t, "primary", sleepPids[0])
	writePid(t, "sidecar", sleepPids[1])
	exitCode, stderr = runInit(t, "stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err = ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, "open var/run/pids.yaml: no such file or directory")

	teardown(t)

	/* 4) Stoppable multi-process partially running service stops. */
	setupMultiProcess(t)

	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	pidBytes, err = exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pid, err = strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)
	writePid(t, "primary", pid)
	exitCode, stderr = runInit(t, "stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err = ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, "open var/run/pids.yaml: no such file or directory")

	teardown(t)

	/* 5) Unstoppable multi-process service does not stop. */
	setupMultiProcess(t)

	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 10000 &").Run())
	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 10000 &").Run())
	pidsBytes, err = exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pidsStrings = strings.Split(string(pidsBytes), "\n")
	sleepPids = make([]int, len(pidsStrings)-1)
	for i, pidString := range pidsStrings[0 : len(pidsStrings)-1] {
		sleepPids[i], err = strconv.Atoi(pidString)
		require.NoError(t, err)
	}
	writePid(t, "primary", sleepPids[0])
	writePid(t, "sidecar", sleepPids[1])
	exitCode, stderr = runInit(t, "stop")

	assert.Equal(t, 1, exitCode)
	// Truncating the expected error message because it contains a nondeterministically ordered list of pids
	assert.Contains(t, stderr, fmt.Sprintf("failed to stop at least one process: failed to wait for all processes to "+
		"stop: processes with pids"))

	pids = readPids(t).PidsByName
	primary, _ := os.FindProcess(pids["primary"])
	sidecar, _ := os.FindProcess(pids["sidecar"])
	require.NoError(t, primary.Signal(syscall.SIGKILL))
	require.NoError(t, sidecar.Signal(syscall.SIGKILL))
}

func TestInitStop_RemovesPidfileNotRunningPidfileExistsSingleProcess(t *testing.T) {
	setupSingleProcess(t)
	defer teardown(t)

	writePid(t, "primary", 99999)
	exitCode, stderr := runInit(t, "stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, "open var/run/pids.yaml: no such file or directory")
}

func TestInitStop_RemovesPidfileNotRunningPidfileExistsMultiProcess(t *testing.T) {
	setupMultiProcess(t)
	defer teardown(t)

	writePid(t, "primary", 99999)
	writePid(t, "sidecar", 99998)
	exitCode, stderr := runInit(t, "stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(pidfile)
	assert.EqualError(t, err, "open var/run/pids.yaml: no such file or directory")
}

func TestInitStop_DoesNothingNotRunningPidfileDoesNotExist(t *testing.T) {
	exitCode, stderr := runInit(t, "stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
}

// Adapted from Stack Overflow: http://stackoverflow.com/questions/10385551/get-exit-code-go
func runInit(t *testing.T, args ...string) (int, string) {
	var errbuf bytes.Buffer
	cli, err := products.Bin("go-init")
	require.NoError(t, err)
	cmd := exec.Command(cli, args...)
	cmd.Stderr = &errbuf
	err = cmd.Run()
	stderr := errbuf.String()

	if err != nil {
		// try to get the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			return ws.ExitStatus(), stderr
		} else {
			// This will happen (in OSX) if `name` is not available in $PATH,
			// in this situation, exit code could not be get, and stderr will be
			// empty string very likely, so we use the default fail code, and format err
			// to string and set to stderr
			log.Printf("Could not get exit code for failed program: %v, %v", cli, args)
			if stderr == "" {
				stderr = err.Error()
			}
			return -1, stderr
		}
	} else {
		// success, exitCode should be 0 if go is ok
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		return ws.ExitStatus(), stderr
	}
}
