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

func statusCommand() cli.Command {
	return cli.Command{
		Name: "status",
		Usage: `
Determines the status of the service defined by the static and custom configurations at service/bin/launcher-static.yml
and var/conf/launcher-custom.yml.
Exits:
- 0 if all of its processes are running
- 1 if at least one process is not running
- 3 if the status cannot be determined
If exit code is nonzero, writes an error message to stderr.`,
		Action: func(_ cli.Context) error {
			return status()
		},
	}
}

func status() error {
	notRunningCmds, err := lib.GetNotRunningCmds()
	if err != nil {
		return logErrorAndReturnWithExitCode(errors.Wrap(err, "failed to determine service status"), 3)
	}
	if len(notRunningCmds) > 0 {
		notRunningCmdNames := make([]string, 0, len(notRunningCmds))
		for name := range notRunningCmds {
			notRunningCmdNames = append(notRunningCmdNames, name)
		}
		return logErrorAndReturnWithExitCode(errors.Errorf("commands '%v' are not running",
			notRunningCmdNames), 1)
	}
	return nil
}
