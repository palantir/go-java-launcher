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
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopProcess_RunningStoppableTerminatesAndRunningUnstoppableDoesNotTerminate(t *testing.T) {
	// Run sleep in sh so that it's not a child process of the one checking if it's running. The echo is to have a
	// fairly surely unique text reference to grep for later (since we don't get the PID of a grandchild process).
	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	pidBytes, err := exec.Command("pgrep", "-f", "sleep").Output()
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)

	process, _ := os.FindProcess(pid)
	println(fmt.Sprintf("k now we're going to stop, pid is '%d'", pid))
	assert.NoError(t, StopProcess(process))

	// Signum 15 is SIGTERM - need a program that ignores SIGTERM and thus won't stop even after waiting.
	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 10000 &").Run())
	pidBytes, err = exec.Command("pgrep", "-f", "sleep").Output()
	require.NoError(t, err)
	pid, err = strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)

	process, _ = os.FindProcess(pid)
	assert.EqualError(t, StopProcess(process), fmt.Sprintf("failed to stop process: failed to wait for process to "+
		"stop: process with pid '%d' did not stop within 240 seconds", pid))

	// Clean up the process
	require.NoError(t, process.Signal(syscall.SIGKILL))
}

func TestStopProcess_NotRunning(t *testing.T) {
	process, _ := os.FindProcess(99999)
	assert.NoError(t, StopProcess(process))
}
