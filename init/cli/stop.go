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

package cli

import (
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/init/lib"
	"github.com/palantir/go-java-launcher/launchlib"
)

var stopCliCommand = cli.Command{
	Name: "stop",
	Usage: `
Ensures the service defined by the static and custom configurations are service/bin/launcher-static.yml and
var/conf/launcher-custom.yml is not running. If successful, exits 0, otherwise exits 1 and writes an error message to
stderr and var/log/startup.log. Waits for at least 240 seconds for any processes to stop.`,
	Action: stop,
}

func stop(ctx cli.Context) (rErr error) {
	outputFile, err := os.OpenFile(launchlib.PrimaryOutputFile, lib.OutputFileFlag, lib.OutputFileMode)
	if err != nil {
		return cli.WithExitCode(1, errors.Errorf("failed to create primary output file: %s",
			launchlib.PrimaryOutputFile))
	}
	defer func() {
		if cErr := outputFile.Close(); rErr == nil && cErr != nil {
			rErr = cli.WithExitCode(1, errors.Errorf("failed to close primary output file: %s",
				launchlib.PrimaryOutputFile))
		}
	}()
	ctx.App.Stdout = outputFile

	runningProcs, err := lib.GetRunningProcs()
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to stop service"), 1)
	}
	if err := stopService(runningProcs); err != nil {
		return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to stop service"), 1)
	}
	return nil
}

func stopService(procs map[string]*os.Process) error {
	for name, proc := range procs {
		if err := proc.Signal(syscall.SIGTERM); err != nil && !strings.Contains(err.Error(),
			"os: process already finished") {
			return errors.Wrapf(err, "failed to stop '%s' process", name)
		}
	}

	if err := waitForServiceToStop(procs); err != nil {
		return errors.Wrap(err, "failed to stop at least one process")
	}

	if err := os.Remove(lib.Pidfile); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove Pidfile")
	}

	return nil
}

func waitForServiceToStop(procs map[string]*os.Process) error {
	// TODO
	const numSecondsToWait = 5
	timer := time.NewTimer(numSecondsToWait * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for name, remainingProc := range procs {
				if !lib.IsProcRunning(remainingProc) {
					delete(procs, name)
				}
			}
			if len(procs) == 0 {
				return nil
			}
		case <-timer.C:
			remainingPids := make(map[string]int, len(procs))
			i := 0
			for name, proc := range procs {
				remainingPids[name] = proc.Pid
				i++
			}
			return errors.Errorf("failed to wait for all processes to stop: processes with pids '%v' did "+
				"not stop within %d seconds", remainingPids, numSecondsToWait)
		}
	}
}
