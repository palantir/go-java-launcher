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
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
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
	waitDuration := 240 * time.Second
	timer := time.NewTimer(waitDuration)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !isRunning(process) {
				return nil
			}
		case <-timer.C:
			return errors.Errorf(
				"failed to wait for process to stop: process with pid '%d' did not stop within %d seconds")
		}
	}
}
