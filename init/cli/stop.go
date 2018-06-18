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
)

func stopCommand() cli.Command {
	return cli.Command{
		Name: "stop",
		Usage: `
Stops the process the PID of which is written to var/run/service.pid. Returns 0 if the process is successfully stopped
or is not running and the pidfile is removed and returns 1 otherwise. Waits 240 seconds for the process to stop before
considering the execution a failure.`,
		Action: func(_ cli.Context) error {
			return stop()
		},
	}
}

func stop() error {
	// The status tells us more than the error
	switch process, status, _ := lib.GetProcessStatus(); status {
	case 0:
		if err := lib.StopProcess(process); err != nil {
			return cli.WithExitCode(1, err)
		}
		if err := os.Remove(lib.Pidfile); err != nil {
			return cli.WithExitCode(1, err)
		}
		return nil
	case 1:
		if err := os.Remove(lib.Pidfile); err != nil {
			return cli.WithExitCode(1, err)
		}
		return nil
	case 3:
		return nil
	default:
		return cli.WithExitCode(1, errors.Errorf("internal error, process status code not a known value: %d", status))
	}
}
