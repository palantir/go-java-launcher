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
	"strconv"
	"syscall"
)

func isRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}
	return true
}

// IsRunningByPidFile determines the status of the process whose PID is contained in the given pid-file. Returns:
// - 0 if pid exists and can be read and the process is running,
// - 1 if the pid-file exists but the process is not running, and
// - 3 if the pid-file does not exist or cannot be read; returns a non-nil error explaining the underlying error.
func IsRunningByPidFile(pidFile string) (int, error) {
	pid, err := GetPid(pidFile)
	if err != nil {
		return 3, err
	}

	if !isRunning(pid) {
		return 1, nil
	}
	return 0, nil
}

func GetPid(pidFile string) (int, error) {
	bytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return -1, err
	}
	pid, err := strconv.Atoi(string(bytes[:]))
	if err != nil {
		return -1, err
	}

	return pid, nil
}
