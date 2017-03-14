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
	"errors"
	"fmt"

	"github.com/palantir/pkg/cli"
	"github.com/palantir/pkg/cli/flag"

	"github.com/palantir/go-java-launcher/init/lib"
)

func statusCommand() cli.Command {
	return cli.Command{
		Name: "status",
		Usage: `
Determines the status of the process whose PID is contained in the given pid-file. Returns 0 if the
process is running, 1 if the pid-file exists but the process is not running, and 3 if the pid-file
does not exist`,
		Flags: []flag.Flag{
			flag.StringFlag{
				Name:  pidfileParameter,
				Usage: "The path to a file containing the PID for which the status is to be determined",
				Value: defaultPidFile},
		},
		Action: doStatus,
	}
}

func doStatus(ctx cli.Context) error {
	pidFile := ctx.String(pidfileParameter)
	isRunning, err := lib.IsRunningByPidFile(pidFile)
	if err != nil {
		msg := fmt.Sprintf("Failed to determine whether process is running for pid-file: %s", pidFile)
		return respondError(msg, err, isRunning)
	}

	switch isRunning {
	case 0:
		pid, _ := lib.GetPid(pidFile)
		return respondSuccess(isRunning, fmt.Sprintf("Running (%d)\n", pid))
	case 1:
		return respondSuccess(isRunning, "Process dead but pidfile exists\n")
	}

	return errors.New("Internal error, failed to determine status")
}
