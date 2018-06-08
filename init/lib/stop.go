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
	"time"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"syscall"
	"strings"
)

func StopProcess(process *os.Process) error {
	if err := process.Signal(syscall.SIGTERM); err != nil {
		if !strings.Contains(err.Error(), "os: process already finished") {
			return errors.Wrap(err, "failed to stop process")
		}
	}

	if err := waitForProcessToStop(process); err != nil {
		return errors.Wrap(err, "failed to stop process")
	}

	return nil
}

func waitForProcessToStop(process *os.Process) error {
	numSecondsToWait := 10
	counter := 0
	for isRunning(process) && counter < numSecondsToWait {
		time.Sleep(time.Second)
		counter++
	}

	if isRunning(process) {
		msg := fmt.Sprintf("failed to wait for process to stop: process with pid '%d' did not stop within %d seconds",
			process.Pid, numSecondsToWait)
		return errors.New(msg)
	}

	return nil
}
