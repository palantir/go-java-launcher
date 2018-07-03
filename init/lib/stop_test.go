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

// All these tests must run sequentially since they all utilize global state (the process table).
func TestStopService_Running(t *testing.T) {

	/* 1) Stoppable single-process service stops. */

	// Run sleep in sh so that it's not a child process of the one checking if it's running.
	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	// -P specifies the PPID to filter on. In this case sleep will be orphaned and adopted by init.
	pidBytes, err := exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)

	proc, _ := os.FindProcess(pid)
	require.NoError(t, StopService([]*os.Process{proc}))

	/* 2) Unstoppable single-process service does not stop. */

	// Signum 15 is SIGTERM - need a program that ignores SIGTERM and thus won't stop even after waiting.
	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 10000 &").Run())
	pidBytes, err = exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pid, err = strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)

	proc, _ = os.FindProcess(pid)
	// TODO
	require.EqualError(t, StopService([]*os.Process{proc}), fmt.Sprintf("failed to stop at least one process: "+
		"failed to wait for all processes to stop: processes with pids '%v' did not stop within 5 seconds",
		[]int{pid}))

	// Clean up the process
	require.NoError(t, proc.Signal(syscall.SIGKILL))

	/* 3) Stoppable multi-process service stops. */

	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	pidsBytes, err := exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pidsStrings := strings.Split(string(pidsBytes), "\n")
	pids := make([]int, len(pidsStrings)-1)
	for i, pidString := range pidsStrings[0 : len(pidsStrings)-1] {
		pids[i], err = strconv.Atoi(pidString)
		require.NoError(t, err)
	}
	procs := make([]*os.Process, len(pids))
	for i, pid := range pids {
		procs[i], _ = os.FindProcess(pid)
	}

	require.NoError(t, StopService(procs))

	/* 4) Unstoppable multi-process service does not stop. */

	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 10000 &").Run())
	unstoppablePidBytes, err := exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	unstoppablePid, err := strconv.Atoi(strings.Split(string(unstoppablePidBytes), "\n")[0])
	require.NoError(t, err)
	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	pidsBytes, err = exec.Command("pgrep", "-f", "-P", "1", "sleep").Output()
	require.NoError(t, err)
	pidsStrings = strings.Split(string(pidsBytes), "\n")
	pids = make([]int, len(pidsStrings)-1)
	for i, pidString := range pidsStrings[0 : len(pidsStrings)-1] {
		pids[i], err = strconv.Atoi(pidString)
		require.NoError(t, err)
	}
	procs = make([]*os.Process, len(pids))
	for i, pid := range pids {
		procs[i], _ = os.FindProcess(pid)
	}

	// TODO
	require.EqualError(t, StopService(procs), fmt.Sprintf("failed to stop at least one process: "+
		"failed to wait for all processes to stop: processes with pids '%v' did not stop within 5 seconds",
		[]int{unstoppablePid}))

	unstoppableProc, _ := os.FindProcess(unstoppablePid)
	require.NoError(t, unstoppableProc.Signal(syscall.SIGKILL))
}

func TestStopProcess_NotRunningSingleProcess(t *testing.T) {
	proc, _ := os.FindProcess(99999)
	assert.NoError(t, StopService([]*os.Process{proc}))
}

func TestStopProcess_NotRunningMultiProcess(t *testing.T) {
	procs := make([]*os.Process, 2)
	procs[0], _ = os.FindProcess(99998)
	procs[1], _ = os.FindProcess(99999)
	assert.NoError(t, StopService(procs))
}
