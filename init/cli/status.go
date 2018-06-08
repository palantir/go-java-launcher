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

	"github.com/palantir/go-java-launcher/init/lib"
)

func statusCommand() cli.Command {
	return cli.Command{
		Name: "status",
		Usage: `
Determines the status of the process the PID of which is written to var/run/service.pid.
Exits:
- 0 if the pidfile exists and can be read and the process is running
- 1 if the pidfile exists and can be read but the process is not running
- 3 if the pidfile does not exist or cannot be read
If exit code is nonzero, writes an error message to stderr.`,
		Action: status,
	}
}

func status(_ cli.Context) error {
	_, status, err := lib.GetProcessStatus()
	if err != nil {
		return cli.WithExitCode(status, err)
	}
	return nil
}
