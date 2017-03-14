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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var stdoutFile *os.File
var testStdoutFile *os.File
var pidFile *os.File

func setup() {
	var err error
	pidFile, err = ioutil.TempFile("", "pid")
	if err != nil {
		panic(err)
	}
	stdoutFile, err = ioutil.TempFile("", "stdout")
	if err != nil {
		panic(err)
	}
	testStdoutFile, err = ioutil.TempFile("", "testStdout")
	if err != nil {
		panic(err)
	}
	os.Stdout = testStdoutFile
}

func TestStart(t *testing.T) {
	setup()
	os.Stdout = testStdoutFile

	cmd := &exec.Cmd{Path: "/bin/ls"}
	pid, err := StartCommandWithOutputRedirectionAndPidFile(cmd, stdoutFile, pidFile.Name())
	assert.NoError(t, err)

	// Assert that output has been written to injected stdoutFile instead of context stdout
	time.Sleep(time.Second) // Wait for forked process to start and print output
	cmdStdout, _ := ioutil.ReadFile(stdoutFile.Name())
	testStdout, _ := ioutil.ReadFile(testStdoutFile.Name())
	assert.Contains(t, string(cmdStdout), "start.go")
	assert.Empty(t, string(testStdout))

	// Assert that pidfile was written
	assert.Equal(t, pid, readPid(pidFile.Name()))
}

func TestStart_DoesNotStartAlreadyRunningService(t *testing.T) {
	setup()
	writePid(pidFile.Name(), os.Getpid())

	cmd := &exec.Cmd{Path: "/bin/ls"}
	pid, err := StartCommandWithOutputRedirectionAndPidFile(cmd, stdoutFile, pidFile.Name())
	assert.NoError(t, err)

	// Assert that command was not run since it's already running
	time.Sleep(time.Second) // Wait for forked process to start and print output
	cmdStdout, _ := ioutil.ReadFile(stdoutFile.Name())
	assert.Empty(t, string(cmdStdout))

	// Assert that pidfile was not overwritten
	assert.Equal(t, os.Getpid(), readPid(pidFile.Name()))
	assert.Equal(t, os.Getpid(), pid)
}

func TestStart_RestartsTheServiceWhenPidFilePidIsStale(t *testing.T) {
	setup()
	deadPid := 99999
	writePid(pidFile.Name(), deadPid)

	cmd := &exec.Cmd{Path: "/bin/ls"}
	pid, err := StartCommandWithOutputRedirectionAndPidFile(cmd, stdoutFile, pidFile.Name())
	assert.NoError(t, err)

	// Assert that command was not run since it's already running
	time.Sleep(time.Second) // Wait for forked process to start and print output
	cmdStdout, _ := ioutil.ReadFile(stdoutFile.Name())
	assert.Contains(t, string(cmdStdout), "start.go")

	// Assert that pidfile was overwritten
	assert.Equal(t, pid, readPid(pidFile.Name()))
	assert.NotEqual(t, deadPid, pid)
}

func writePid(fileName string, pid int) {
	err := ioutil.WriteFile(fileName, []byte(strconv.Itoa(pid)), 0644)
	if err != nil {
		panic(err)
	}
}

func readPid(fileName string) int {
	writtenPid, _ := ioutil.ReadFile(fileName)
	writtenPidAsInt, _ := strconv.Atoi(string(writtenPid))
	return writtenPidAsInt
}
