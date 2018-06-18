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

	"github.com/pkg/errors"
)

// GetProcessStatus determines the status of the process whose PID is written to var/run/service.pid.
//
// Returns (process, status, err). Possible values are:
// - (<process>, 0, nil) if the pidfile exists and can be read and the process is running
// - (<process>, 1, <err>) if the pidfile exists and can be read but the process is not running
// - (nil, 3, <err>) if the pidfile does not exist or cannot be read
func GetProcessStatus() (*os.Process, int, error) {
	pidBytes, err := ioutil.ReadFile(Pidfile)
	if err != nil {
		return nil, 3, errors.Wrap(err, "failed to read pidfile")
	}

	pid, err := strconv.Atoi(string(pidBytes[:]))
	if err != nil {
		return nil, 3, errors.Wrap(err, "failed to parse a valid PID from pidfile")
	}

	// Docs say FindProcess always succeeds on Unix.
	process, _ := os.FindProcess(pid)
	if !isRunning(process) {
		return process, 1, errors.New("pidfile exists but process is not running")
	}

	return process, 0, nil
}

func isRunning(process *os.Process) bool {
	return process.Signal(syscall.Signal(0)) == nil
}
