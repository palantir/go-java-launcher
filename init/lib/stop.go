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

func StopService(procs []*os.Process) error {
	for _, proc := range procs {
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			if !strings.Contains(err.Error(), "os: process already finished") {
				return errors.Wrap(err, "failed to stop at least one process")
			}
		}
	}

	if err := waitForServiceToStop(procs); err != nil {
		return errors.Wrap(err, "failed to stop at least one process")
	}

	if err := os.Remove(pidfile); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove pidfile")
	}

	return nil
}

func waitForServiceToStop(procs []*os.Process) error {
	// TODO
	const numSecondsToWait = 5
	timer := time.NewTimer(numSecondsToWait * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	remainingProcs := make(map[*os.Process]struct{})
	for _, proc := range procs {
		remainingProcs[proc] = struct{}{}
	}
	for {
		select {
		case <-ticker.C:
			for remainingProc := range remainingProcs {
				if !isProcRunning(remainingProc) {
					delete(remainingProcs, remainingProc)
				}
			}
			if len(remainingProcs) == 0 {
				return nil
			}
		case <-timer.C:
			remainingPids := make([]int, len(remainingProcs))
			i := 0
			for proc := range remainingProcs {
				remainingPids[i] = proc.Pid
				i++
			}
			return errors.Errorf("failed to wait for all processes to stop: processes with pids '%v' did "+
				"not stop within %d seconds", remainingPids, numSecondsToWait)
		}
	}
}
