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
	"os"

	"github.com/palantir/pkg/cli"
	"github.com/palantir/pkg/cli/flag"

	"github.com/palantir/go-java-launcher/init/lib"
	"github.com/palantir/go-java-launcher/launchlib"
)

func startCommand() cli.Command {
	return cli.Command{
		Name: "start",
		Usage: `
Runs the command defined by the given static and custom configurations and stores the PID of the resulting process in
the given pid file.
`,
		Flags: []flag.Flag{
			flag.StringFlag{
				Name:  launcherStaticFileParameter,
				Value: "service/bin/launcher-static.yml",
				Usage: "The location of the LauncherStatic file configuration the started command"},
			flag.StringFlag{
				Name:  launcherCustomFileParameter,
				Value: "var/conf/launcher-custom.yml",
				Usage: "The location of the LauncherCustom file configuration the started command"},
			flag.StringFlag{
				Name:  pidfileParameter,
				Value: "var/run/service.pid",
				Usage: "The location of the file storing the process ID of the started command"},
			flag.StringFlag{
				Name:  outFileParameter,
				Value: "var/log/startup.log",
				Usage: "The location of the file to which STDOUT and STDERR of the started command are redirected"},
		},
		Action: doStart,
	}
}

func doStart(ctx cli.Context) error {
	launcherStaticFile := ctx.String(launcherStaticFileParameter)
	launcherCustomFile := ctx.String(launcherCustomFileParameter)
	pidfile := ctx.String(pidfileParameter)
	stdoutfileName := ctx.String(outFileParameter)

	stdoutfile, err := os.Create(stdoutfileName)
	if err != nil {
		msg := fmt.Sprintln("Failed to create startup log file", err)
		return respondError(msg, err, 1)
	}

	originalStdout := os.Stdout
	os.Stdout = stdoutfile // log command assembly output to file instead of stdout
	cmd, err := launchlib.CompileCmdFromConfigFiles(launcherStaticFile, launcherCustomFile)
	if err != nil {
		msg := fmt.Sprintln("Failed to assemble Command object from static and custom configuration files", err)
		return respondError(msg, err, 1)
	}
	os.Stdout = originalStdout

	_, err = lib.StartCommandWithOutputRedirectionAndPidFile(cmd, stdoutfile, pidfile)
	if err != nil {
		msg := fmt.Sprintln("Failed to start process", err)
		return respondError(msg, err, 1)
	}

	return respondSuccess(0)
}
