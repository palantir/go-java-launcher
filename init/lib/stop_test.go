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
	proc, _ := os.FindProcess(pgrep(t, "sleep"))
	require.NoError(t, StopService(map[string]*os.Process{"primary": proc}))

	/* 2) Unstoppable single-process service does not stop. */

	// Signum 15 is SIGTERM - need a program that ignores SIGTERM and thus won't stop even after waiting.
	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 10000 &").Run())
	pid := pgrep(t, "sleep")
	proc, _ = os.FindProcess(pid)
	// TODO
	require.EqualError(t, StopService(map[string]*os.Process{"primary": proc}), fmt.Sprintf("failed to stop "+
		"'primary' process: failed to wait for all processes to stop: processes with pids '%v' did not stop "+
		"within 5 seconds", map[string]int{"primary": pid}))
	// Clean up the process
	require.NoError(t, proc.Signal(syscall.SIGKILL))

	/* 3) Stoppable multi-process service stops. */

	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 9999 &").Run())
	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	procs := make(map[string]*os.Process)
	procs["primary"], _ = os.FindProcess(pgrep(t, "sleep 9999"))
	procs["sidecar"], _ = os.FindProcess(pgrep(t, "sleep 10000"))
	require.NoError(t, StopService(map[string]*os.Process{}))

	/* 4) Unstoppable multi-process service does not stop. */

	require.NoError(t, exec.Command("/bin/sh", "-c", "trap '' 15; /bin/sleep 9999 &").Run())
	require.NoError(t, exec.Command("/bin/sh", "-c", "/bin/sleep 10000 &").Run())
	unstoppablePid := pgrep(t, "sleep 9999")
	procs = make(map[string]*os.Process)
	procs["primary"], _ = os.FindProcess(unstoppablePid)
	procs["sidecar"], _ = os.FindProcess(pgrep(t, "sleep 10000"))
	// TODO
	require.EqualError(t, StopService(procs), fmt.Sprintf("failed to stop at least one process: "+
		"failed to wait for all processes to stop: processes with pids '%v' did not stop within 5 seconds",
		[]int{unstoppablePid}))
	require.NoError(t, procs["primary"].Signal(syscall.SIGKILL))
}

func TestStopProcess_NotRunningSingleProcess(t *testing.T) {
	proc, _ := os.FindProcess(99999)
	assert.NoError(t, StopService(map[string]*os.Process{"primary": proc}))
}

func TestStopProcess_NotRunningMultiProcess(t *testing.T) {
	procs := make(map[string]*os.Process)
	procs["primary"], _ = os.FindProcess(99998)
	procs["sidecar"], _ = os.FindProcess(99999)
	assert.NoError(t, StopService(procs))
}

func pgrep(t *testing.T, key string) int {
	// -P specifies the PPID to filter on. In these tests the processes are orphaned and adopted by init.
	pidBytes, err := exec.Command("pgrep", "-f", "-P", "1", key).Output()
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.Split(string(pidBytes), "\n")[0])
	require.NoError(t, err)
	return pid
}
