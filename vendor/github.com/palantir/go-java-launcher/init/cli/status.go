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

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/launchlib"
)

var statusCliCommand = cli.Command{
	Name: "status",
	Usage: `
Determines the status of the service defined by the static and custom configurations at service/bin/launcher-static.yml
and var/conf/launcher-custom.yml.
Exits:
- 0 if all of its processes are running
- 1 if at least one process is not running but there is a record of processes having been started
- 3 if no processes are running and there is no record of processes having been started
- 4 if the status cannot be determined
If exit code is nonzero, writes an error message to stderr and var/log/startup.log.`,
	Action: executeWithLoggers(status, NewAlwaysAppending()),
}

var (
	Running = ServiceState{
		Description: "Running",
		Applicable: func(serviceStatus *serviceStatus, err error) bool {
			return err == nil && len(serviceStatus.notRunningCmds) == 0
		},
		ExitStatus: func(serviceStatus *serviceStatus, err error) (int, error) {
			return 0, nil
		},
	}
	Dead = ServiceState{
		Description: "Process dead but pidfile exists.",
		Applicable: func(serviceStatus *serviceStatus, err error) bool {
			return err == nil && len(serviceStatus.notRunningCmds) > 0 && len(serviceStatus.writtenPids) > 0
		},
		ExitStatus: func(serviceStatus *serviceStatus, err error) (int, error) {
			return 1, errors.Errorf("commands '%v' are not running but there is a record of commands '%v' "+
				"having been started", commandNames(serviceStatus.notRunningCmds), serviceStatus.writtenPids)
		},
	}
	NotRunning = ServiceState{
		Description: "Service not running",
		Applicable: func(serviceStatus *serviceStatus, err error) bool {
			return err == nil && len(serviceStatus.notRunningCmds) > 0 && len(serviceStatus.writtenPids) == 0
		},
		ExitStatus: func(serviceStatus *serviceStatus, err error) (int, error) {
			return 3, errors.Errorf("commands '%v' are not running", commandNames(serviceStatus.notRunningCmds))
		},
	}
	ErrorState = ServiceState{
		Description: "Failed to determine service status",
		Applicable: func(serviceStatus *serviceStatus, err error) bool {
			return err != nil
		},
		ExitStatus: func(serviceStatus *serviceStatus, err error) (int, error) {
			if err != nil {
				return 4, errors.Wrap(err, "failed to determine service status")
			}
			return 4, errors.Errorf("failed to determine service status")
		},
	}
)

func status(ctx cli.Context, loggers launchlib.ServiceLoggers) error {
	// Executed with logging for errors, however we discard the verbose logging of getServiceStatus
	serviceStatus, err := getServiceStatus(ctx, &DevNullLoggers{})
	var matched *ServiceState
	for _, state := range []ServiceState{ErrorState, NotRunning, Dead, Running} {
		if state.Applicable(serviceStatus, err) {
			matched = &state
			break
		}
	}

	// If no state has matched, default to error state
	if matched == nil {
		matched = &ErrorState
	}

	code, err := matched.ExitStatus(serviceStatus, err)
	if code != 0 {
		fmt.Fprintln(os.Stderr, matched.Description)
		if err != nil {
			return logErrorAndReturnWithExitCode(ctx, err, code)
		}
		// Non-zero exit codes can only be reported through cli.WithExitCode which must include an errors, though that
		// error can be empty.  Returning no error gives an exit code of 0
		return cli.WithExitCode(code, errors.New(""))
	}

	fmt.Println(matched.Description)
	return nil
}

type ServiceState struct {
	Description string
	Applicable  func(serviceStatus *serviceStatus, err error) bool
	ExitStatus  func(serviceStatus *serviceStatus, err error) (int, error)
}

func commandNames(commands map[string]CommandContext) []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	return names
}
