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
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"

	time2 "github.com/palantir/go-java-launcher/init/cli/time"
	"github.com/palantir/go-java-launcher/launchlib"
)

var (
	// Clock is overridden in the tests to be a fake clock
	Clock = time2.NewRealClock()
)

var stopCliCommand = cli.Command{
	Name: "stop",
	Usage: `
Ensures the service defined by the static and custom configurations are service/bin/launcher-static.yml and
var/conf/launcher-custom.yml is not running. If successful, exits 0, otherwise exits 1 and writes an error message to
stderr and var/log/startup.log. Waits for at least 240 seconds for any processes to stop before sending a SIGKILL.`,
	Action: executeWithLoggers(stop, NewAlwaysAppending()),
}

func stop(ctx cli.Context, loggers launchlib.ServiceLoggers) error {
	cmds, err := getConfiguredCommands(ctx, loggers)
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx,
			errors.Wrap(err, "failed to get commands from static and custom configuration files"), 1)
	}

	runningProcs := map[string]*os.Process{}
	for name := range cmds {
		_, proc, err := getCmdProcess(name)
		if err != nil {
			return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to determine process status"), 1)
		}

		if proc != nil {
			runningProcs[name] = proc
		}
	}

	if err := stopService(ctx, runningProcs); err != nil {
		return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to stop service"), 1)
	}

	var errs bool
	for name := range cmds {
		if err := os.Remove(fmt.Sprintf(pidfileFormat, name)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(ctx.App.Stderr, "failed to remove stopped process pidfile for '%s'\n", name)
			errs = true
		}
	}

	if errs {
		return logErrorAndReturnWithExitCode(ctx, errors.New("error removing stopped service pidfiles"), 1)
	}
	return nil
}

func stopService(ctx cli.Context, procs map[string]*os.Process) error {
	for name, proc := range procs {
		if err := proc.Signal(syscall.SIGTERM); err != nil && !strings.Contains(err.Error(),
			"os: process already finished") {
			return errors.Wrapf(err, "failed to stop '%s' process", name)
		}
	}

	if err := waitForServiceToStop(ctx, procs); err != nil {
		return errors.Wrap(err, "failed to stop at least one process")
	}

	return nil
}

func waitForServiceToStop(ctx cli.Context, procs map[string]*os.Process) error {
	const numSecondsToWait = 240
	timer := Clock.NewTimer(numSecondsToWait * time.Second)
	defer timer.Stop()

	ticker := Clock.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			for name, remainingProc := range procs {
				if !isProcRunning(remainingProc) {
					delete(procs, name)
				}
			}
			if len(procs) == 0 {
				return nil
			}
		case <-timer.Chan():
			killedProcs := make([]string, 0, len(procs))
			for name, remainingProc := range procs {
				if isProcRunning(remainingProc) {
					if err := remainingProc.Kill(); err != nil {
						// If this actually errors, something is probably seriously wrong.
						// Just stop immediately.
						return errors.Wrapf(err, "failed to kill process with pid %d",
							remainingProc.Pid)
					}
					killedProcs = append(killedProcs, name)
				}
			}
			fmt.Fprintf(ctx.App.Stdout, "processes '%v' did not stop within %d seconds, so a SIGKILL was "+
				"sent", killedProcs, numSecondsToWait)
			return nil
		}
	}
}
