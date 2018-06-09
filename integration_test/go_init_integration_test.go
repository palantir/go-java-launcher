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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"github.com/palantir/godel/pkg/products/v2/products"
	"github.com/stretchr/testify/assert"

	"time"
	"bytes"
	"github.com/palantir/go-java-launcher/init/lib"
	"strconv"
	"strings"
	"fmt"
)

func setup() {
	lib.Setup("testdata/launcher-static.yml", "testdata/launcher-custom.yml")
}

func TestInitStart_DoesNotRestartRunning(t *testing.T) {
	setup()
	defer lib.Teardown()

	lib.WritePid(os.Getpid())
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, os.Getpid(), lib.ReadPid())
	assert.Empty(t, stderr)
}

func TestInitStart_StartsNotRunningPidfileExists(t *testing.T) {
	setup()
	defer lib.Teardown()

	lib.WritePid(99999)
	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second) // Wait for JVM to start and print output
	startupLogBytes, err := ioutil.ReadFile(lib.OutputFile)
	if err != nil {
		panic(err)
	}
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, stderr)
}

func TestInitStart_StartsNotRunningPidfileDoesNotExist(t *testing.T) {
	setup()
	defer lib.Teardown()

	exitCode, stderr := runInit("start")

	assert.Equal(t, 0, exitCode)
	time.Sleep(time.Second) // Wait for JVM to start and print output
	startupLogBytes, err := ioutil.ReadFile(lib.OutputFile)
	if err != nil {
		panic(err)
	}
	startupLog := string(startupLogBytes)
	assert.Contains(t, startupLog, "Using JAVA_HOME")
	assert.Contains(t, startupLog, "main method")
	assert.Empty(t, stderr)
}

func TestInitStatus_Running(t *testing.T) {
	setup()
	defer lib.Teardown()

	lib.WritePid(os.Getpid())
	exitCode, stderr := runInit("status")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
}

func TestInitStatus_NotRunningPidfileExists(t *testing.T) {
	setup()
	defer lib.Teardown()

	lib.WritePid(99999)
	exitCode, stderr := runInit("status")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr, "pidfile exists but process is not running")
}

func TestInitStatus_NotRunningPidfileDoesNotExist(t *testing.T) {
	setup()
	defer lib.Teardown()

	exitCode, stderr := runInit("status")

	assert.Equal(t, 3, exitCode)
	assert.Contains(t, stderr, "failed to read pidfile: open var/run/service.pid: no such file or directory")
}

func TestInitStop_StopsRunning(t *testing.T) {
	setup()
	defer lib.Teardown()

	stoppableCommand := "/bin/echo go-init-testing && /bin/sleep 10000 &"
	if err := exec.Command("/bin/sh", "-c", stoppableCommand).Run(); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)
	pidBytes, err := exec.Command("pgrep", "-f", "go-init-testing").Output()
	if err != nil {
		panic(err)
	}
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	if err != nil {
		panic(err)
	}
	lib.WritePid(pid)
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err = ioutil.ReadFile(lib.Pidfile)
	assert.EqualError(t, err, "open var/run/service.pid: no such file or directory")
}

func TestInitStop_FailsRunningDoesNotTerminate(t *testing.T) {
	setup()
	defer lib.Teardown()

	unstoppableCommand := "trap '' 15; /bin/echo go-init-testing && /bin/sleep 10000 &"
	if err := exec.Command("/bin/sh", "-c", unstoppableCommand).Run(); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)
	pidBytes, err := exec.Command("pgrep", "-f", "go-init-testing").Output()
	if err != nil {
		panic(err)
	}
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	if err != nil {
		panic(err)
	}
	lib.WritePid(pid)
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 1, exitCode)
	msg := fmt.Sprintf("failed to stop process: failed to wait for process to stop: process with pid '%d' did not " +
		"stop within 10 seconds", pid)
	assert.Contains(t, stderr, msg)

	process, _ := os.FindProcess(lib.ReadPid())
	if err := process.Signal(syscall.SIGKILL); err != nil {
		panic(err)
	}
}

func TestInitStop_RemovesPidfileNotRunningPidfileExists(t *testing.T) {
	setup()
	defer lib.Teardown()

	lib.WritePid(99999)
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
	_, err := ioutil.ReadFile(lib.Pidfile)
	assert.EqualError(t, err, "open var/run/service.pid: no such file or directory")
}

func TestInitStop_DoesNothingNotRunningPidfileDoesNotExist(t *testing.T) {
	exitCode, stderr := runInit("stop")

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, stderr)
}

// Adapted from Stack Overflow: http://stackoverflow.com/questions/10385551/get-exit-code-go
func runInit(args ...string) (int, string) {
	var errbuf bytes.Buffer
	cli, err := products.Bin("go-init")
	if err != nil {
		panic(err)
	}
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
