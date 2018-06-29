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

func stopCommand() cli.Command {
	return cli.Command{
		Name: "stop",
		Usage: `
Ensures the service defined by the static and custom configurations are service/bin/launcher-static.yml and
var/conf/launcher-custom.yml is not running. If successful, exits 0, otherwise exits 1 and writes an error message to
stderr. Waits for at least 240 seconds for any processes to stop.`,
		Action: func(_ cli.Context) error {
			return stop()
		},
	}
}

func stop() error {
	// We take action here based on the status, not the error
	switch info, status, _ := lib.GetServiceStatus(); status {
	case 0:
		fallthrough
	case 1:
		if err := lib.StopService(info.RunningProcs); err != nil {
			return cli.WithExitCode(1, err)
		}
		return nil
	case 3:
		return nil
	default:
		return cli.WithExitCode(1, errors.Errorf("internal error, process status code not a known value: %d", status))
	}
}
