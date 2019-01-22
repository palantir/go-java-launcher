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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/launchlib"
)

var startCliCommand = cli.Command{
	Name: "start",
	Usage: `
Ensures the service defined by the static and custom configurations at service/bin/launcher-static.yml and
var/conf/launcher-custom.yml is running and its outputs are redirecting to var/log/startup.log and other
var/log/${SUB_PROCESS}-startup.log files. If successful, exits 0, otherwise exits 1 and writes an error message to
stderr and var/log/startup.log.`,
	Action: executeWithLoggers(start, NewTruncatingFirst()),
}

func start(ctx cli.Context, loggers launchlib.ServiceLoggers) error {
	serviceStatus, err := getServiceStatus(ctx, loggers)
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx,
			errors.Wrap(err, "failed to determine service status to determine what commands to run"), 1)
	}
	if err := startService(ctx, serviceStatus.notRunningCmds); err != nil {
		return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to start service"), 1)
	}
	return nil
}

func startService(ctx cli.Context, notRunningCmds map[string]CommandContext) error {
	for name, cmd := range notRunningCmds {
		if err := startCommand(ctx, cmd); err != nil {
			return errors.Wrapf(err, "failed to start command '%s'", name)
		}

		pidfile := fmt.Sprintf(pidfileFormat, name)
		if err := os.MkdirAll(filepath.Dir(pidfile), 0755); err != nil {
			return errors.Wrapf(err, "unable to create pidfile directory.")
		}

		if err := ioutil.WriteFile(pidfile, []byte(strconv.Itoa(cmd.Command.Process.Pid)), 0644); err != nil {
			return errors.Wrapf(err, "failed to save pid to file for command '%s'", name)
		}
	}
	return nil
}

func startCommand(ctx cli.Context, cmdCtx CommandContext) error {
	if err := launchlib.MkDirs(cmdCtx.Dirs, ctx.App.Stdout); err != nil {
		return errors.Wrap(err, "failed to create directories")
	}

	logger, err := cmdCtx.Logger()
	if err != nil {
		return err
	}
	defer func() {
		if cErr := logger.Close(); cErr != nil {
			fmt.Fprintf(ctx.App.Stdout, "failed to close logger for command")
		}
	}()
	cmdCtx.Command.Stdout = logger
	cmdCtx.Command.Stderr = logger
	if err := cmdCtx.Command.Start(); err != nil {
		return errors.Wrap(err, "failed to start command")
	}
	return nil
}
