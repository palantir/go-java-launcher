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
	"os"
	"github.com/pkg/errors"
	"fmt"
)

func stopCommand() cli.Command {
	return cli.Command{
		Name: "stop",
		Usage: `
Stops the process the PID of which is written to var/run/service.pid. Returns 0 if the process is successfully stopped
or is not running and returns 1 if the process is not successfully stopped.`,
		Action: stop,
	}
}

func stop(_ cli.Context) error {
	// The status tells us more than the error
	process, status, _ := lib.GetProcessStatus()

	switch status {
	case 0:
		if err := lib.StopProcess(process); err != nil {
			return cli.WithExitCode(1, err)
		}
		os.Remove(lib.Pidfile)
		return nil
	case 1:
		os.Remove(lib.Pidfile)
		return nil
	case 3:
		return nil
	default:
		msg := fmt.Sprintf("internal error, process status code not a known value: %d", status)
		err := errors.New(msg)
		return cli.WithExitCode(1, err)
	}
}
