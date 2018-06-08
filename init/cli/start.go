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
	"github.com/palantir/go-java-launcher/init/lib"
	"github.com/palantir/go-java-launcher/launchlib"
	"github.com/pkg/errors"
)

func startCommand() cli.Command {
	return cli.Command{
		Name: "start",
		Usage: `
Launches the process defined by the static and custom configurations at service/bin/launcher-static.yml and
var/conf/launcher-custom.yml. Writes its PID to var/run/service.pid and redirects its output to var/log/startup.log. If
successful, exits 0, otherwise exits 1 and writes an error message to stderr.`,
		Action: start,
	}
}

func start(_ cli.Context) error {
	_, _, err := lib.GetProcessStatus()
	if err == nil {
		// Process already running, don't restart it.
		return nil
	}

	outputFile, err := os.Create(lib.OutputFile)
	if err != nil {
		err = errors.Wrap(err, "failed to create startup log file")
		return cli.WithExitCode(1, err)
	}

	cmd, err := launchlib.CompileCmdFromConfigFiles(lib.LauncherStaticFile, lib.LauncherCustomFile, outputFile)
	if err != nil {
		err = errors.Wrap(err, "failed to assemble command from static and custom configuration files")
		return cli.WithExitCode(1, err)
	}

	if err = lib.StartCommand(cmd, outputFile); err != nil {
		err = errors.Wrap(err, "failed to start process")
		return cli.WithExitCode(1, err)
	}

	return nil
}
