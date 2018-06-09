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
	"testing"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"fmt"
	"strconv"
	"strings"
	"os"
	"syscall"
)

func TestStopProcess_RunningTerminates(t *testing.T) {
	// Run sleep in sh so that it's not a child process of the one checking if it's running. The echo is to have a
	// fairly surely unique text reference to grep for later (since we don't get the PID of a grandchild process).
	stoppableCommand := "/bin/echo go-init-testing && /bin/sleep 10000 &"
	if err := exec.Command("/bin/sh", "-c", stoppableCommand).Run(); err != nil {
		panic(err)
	}
	pidBytes, err := exec.Command("pgrep", "-f", "go-init-testing").Output()
	if err != nil {
		panic(err)
	}
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	if err != nil {
		panic(err)
	}

	process, _ := os.FindProcess(pid)
	assert.NoError(t, StopProcess(process))
}

func TestStopProcess_RunningDoesNotTerminate(t *testing.T) {
	// Signum 15 is SIGTERM - need a program that ignores SIGTERM and thus won't stop even after waiting.
	unstoppableCommand := "trap '' 15; /bin/echo go-init-testing && /bin/sleep 10000 &"
	if err := exec.Command("/bin/sh", "-c", unstoppableCommand).Run(); err != nil {
		panic(err)
	}
	pidBytes, err := exec.Command("pgrep", "-f", "go-init-testing").Output()
	if err != nil {
		panic(err)
	}
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	if err != nil {
		panic(err)
	}

	process, _ := os.FindProcess(pid)
	msg := fmt.Sprintf("failed to stop process: failed to wait for process to stop: process with pid '%d' did not " +
		"stop within 240 seconds", pid)
	assert.EqualError(t, StopProcess(process), msg)

	// Clean up the process
	if err := process.Signal(syscall.SIGKILL); err != nil {
		panic(err)
	}
}

func TestStopProcess_NotRunning(t *testing.T) {
	process, _ := os.FindProcess(99999)
	assert.NoError(t, StopProcess(process))
}
