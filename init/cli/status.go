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

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/init/lib"
	"github.com/palantir/go-java-launcher/launchlib"
)

var statusCliCommand = cli.Command{
	Name: "status",
	Usage: `
Determines the status of the service defined by the static and custom configurations at service/bin/launcher-static.yml
and var/conf/launcher-custom.yml.
Exits:
- 0 if all of its processes are running
- 1 if at least one process is not running
- 3 if the status cannot be determined
If exit code is nonzero, writes an error message to stderr and var/log/startup.log.`,
	Action: status,
}

func status(ctx cli.Context) (rErr error) {
	outputFile, err := os.OpenFile(launchlib.PrimaryOutputFile, lib.OutputFileFlag, lib.OutputFileMode)
	if err != nil {
		return cli.WithExitCode(3, errors.Errorf("failed to create primary output file: %s",
			launchlib.PrimaryOutputFile))
	}
	defer func() {
		if cErr := outputFile.Close(); rErr == nil && cErr != nil {
			rErr = cli.WithExitCode(3, errors.Errorf("failed to close primary output file: %s",
				launchlib.PrimaryOutputFile))
		}
	}()
	ctx.App.Stdout = outputFile

	serviceStatus, err := lib.GetServiceStatus(ctx)
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to determine service status"), 3)
	}
	if len(serviceStatus.NotRunningCmds) > 0 {
		notRunningCmdNames := make([]string, 0, len(serviceStatus.NotRunningCmds))
		for name := range serviceStatus.NotRunningCmds {
			notRunningCmdNames = append(notRunningCmdNames, name)
		}
		return logErrorAndReturnWithExitCode(ctx, errors.Errorf("commands '%v' are not running",
			notRunningCmdNames), 1)
	}
	return nil
}
