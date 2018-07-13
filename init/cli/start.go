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
	"io/ioutil"
	"os"

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
	Action: start,
}

func start(ctx cli.Context) (rErr error) {
	outputFile, err := os.OpenFile(launchlib.PrimaryOutputFile, outputFileFlag, outputFileMode)
	if err != nil {
		return cli.WithExitCode(1, errors.Errorf("failed to create primary output file: %s",
			launchlib.PrimaryOutputFile))
	}
	defer func() {
		if cErr := outputFile.Close(); rErr == nil && cErr != nil {
			rErr = cli.WithExitCode(1, errors.Errorf("failed to close primary output file: %s",
				launchlib.PrimaryOutputFile))
		}
	}()
	ctx.App.Stdout = outputFile

	serviceStatus, err := getServiceStatus(ctx)
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx,
			errors.Wrap(err, "failed to determine service status to determine what commands to run"), 1)
	}
	pids, err := startService(serviceStatus.notRunningCmds)
	if err != nil {
		return logErrorAndReturnWithExitCode(ctx, errors.Wrap(err, "failed to start service"), 1)
	}
	for name, runningProc := range serviceStatus.runningProcs {
		pids[name] = runningProc.Pid
	}
	if err := writePids(pids); err != nil {
		return logErrorAndReturnWithExitCode(ctx,
			errors.Wrap(err, "failed to record pids when starting service"), 1)
	}
	return nil
}

func startService(notRunningCmds map[string]launchlib.CmdWithOutputFileName) (map[string]int, error) {
	pids := make(map[string]int)
	for name, cmd := range notRunningCmds {
		if err := startCommand(cmd); err != nil {
			return nil, errors.Wrapf(err, "failed to start command '%s'", name)
		}
		pids[name] = cmd.Cmd.Process.Pid
	}
	return pids, nil
}

func startCommand(cmd launchlib.CmdWithOutputFileName) (rErr error) {
	stdout, err := os.OpenFile(cmd.OutputFileName, outputFileFlag, outputFileMode)
	if err != nil {
		return errors.Wrap(err, "failed to open output file: "+cmd.OutputFileName)
	}
	defer func() {
		if cErr := stdout.Close(); rErr == nil && cErr != nil {
			rErr = errors.Wrap(err, "failed to close output file: "+cmd.OutputFileName)
		}
	}()
	cmd.Cmd.Stdout = stdout
	cmd.Cmd.Stderr = stdout
	if err := cmd.Cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command")
	}
	return nil
}

func writePids(pids map[string]int) error {
	servicePids := servicePids{Pids: make(map[string]int)}
	for name, pid := range pids {
		servicePids.Pids[name] = pid
	}
	servicePidsBytes, err := yaml.Marshal(servicePids)
	if err != nil {
		return errors.Wrap(err, "failed to serialize pidfile")
	}
	if err := ioutil.WriteFile(pidfile, servicePidsBytes, 0666); err != nil {
		return errors.Wrap(err, "failed to write pidfile")
	}
	return nil
}
