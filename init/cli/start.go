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

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/palantir/go-java-launcher/launchlib"
)

var startCliCommand = cli.Command{
	Name: "start",
	Usage: `
Ensures the service defined by the static and custom configurations at service/bin/launcher-static.yml and
var/conf/launcher-custom.yml is running and its outputs are redirecting to var/log/startup.log and other
var/log/${SUB_PROCESS}-startup.log files. If successful, exits 0, otherwise exits 1 and writes an error message to
stderr and var/log/startup.log.`,
	Action: executeWithContext(start, truncOutputFileFlag),
}

func start(ctx cli.Context) (rErr error) {
	serviceStatus, err := getServiceStatus(ctx)
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx,
			errors.Wrap(err, "failed to determine service status to determine what commands to run"), 1)
	}
	servicePids, err := startService(ctx, serviceStatus.notRunningCmds)
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to start service"), 1)
	}
	for name, runningProc := range serviceStatus.runningProcs {
		servicePids[name] = runningProc.Pid
	}
	if err := writePids(servicePids); err != nil {
		return logErrorAndReturnWithExitCode(ctx,
			errors.Wrap(err, "failed to record pids when starting service"), 1)
	}
	return nil
}

func startService(ctx cli.Context, notRunningCmds map[string]launchlib.CmdWithContext) (servicePids, error) {
	servicePids := servicePids{}
	for name, cmd := range notRunningCmds {
		if err := startCommand(ctx, cmd); err != nil {
			return nil, errors.Wrapf(err, "failed to start command '%s'", name)
		}
		servicePids[name] = cmd.Cmd.Process.Pid
	}
	return servicePids, nil
}

func startCommand(ctx cli.Context, cmd launchlib.CmdWithContext) error {
	if err := launchlib.MkDirs(cmd.Dirs, ctx.App.Stdout); err != nil {
		return errors.Wrap(err, "failed to create directories")
	}
	stdout, err := os.OpenFile(cmd.OutputFileName, appendOutputFileFlag, outputFileMode)
	if err != nil {
		return errors.Wrapf(err, "failed to open output file: %s", cmd.OutputFileName)
	}
	defer func() {
		if cErr := stdout.Close(); cErr != nil {
			fmt.Fprintf(ctx.App.Stdout, "failed to close output file: %s", cmd.OutputFileName)
		}
	}()
	cmd.Cmd.Stdout = stdout
	cmd.Cmd.Stderr = stdout
	if err := cmd.Cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command")
	}
	return nil
}

func writePids(servicePids servicePids) error {
	servicePidsBytes, err := yaml.Marshal(servicePids)
	if err != nil {
		return errors.Wrap(err, "failed to serialize pidfile")
	}
	if err := os.MkdirAll(filepath.Dir(pidfile), 0755); err != nil {
		return cli.WithExitCode(1, errors.Errorf("failed to mkdir for pidfile: %s", pidfile))
	}
	if err := ioutil.WriteFile(pidfile, servicePidsBytes, 0666); err != nil {
		return errors.Wrap(err, "failed to write pidfile")
	}
	return nil
}
