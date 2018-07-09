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
	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/init/lib"
)

func startCommand() cli.Command {
	return cli.Command{
		Name: "start",
		Usage: `
Ensures the service defined by the static and custom configurations at service/bin/launcher-static.yml and
var/conf/launcher-custom.yml is running and its outputs are redirecting to var/log/startup.log and other
var/log/${SUB_PROCESS}-startup.log files. If successful, exits 0, otherwise exits 1 and writes an error message to
stderr.`,
		Action: func(_ cli.Context) error {
			return start()
		},
	}
}

func start() error {
	notRunningCmds, err := lib.GetNotRunningCmds()
	if err != nil {
		// If it can't be determined which are running versus not, fall back to just starting all of them
		notRunningCmds, err = lib.GetConfiguredCommands()
		if err != nil {
			return logErrorAndReturnWithExitCode(errors.Wrap(err, "failed to determine commands to run"), 1)
		}
	}
	if err := lib.StartService(notRunningCmds); err != nil {
		return logErrorAndReturnWithExitCode(errors.Wrap(err, "failed to start service"), 1)
	}
	return nil
}
