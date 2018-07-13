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
	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/launchlib"
)

func App() *cli.App {
	app := cli.NewApp()
	app.Name = "go-init"
	app.Usage = "A simple init.sh-style service launcher CLI."

	app.Subcommands = []cli.Command{startCliCommand, statusCliCommand, stopCliCommand}
	return app
}

func executeWithContext(action func(cli.Context) error) func(cli.Context) error {
	return func(ctx cli.Context) (rErr error) {
		outputFile, err := os.OpenFile(launchlib.PrimaryOutputFile, outputFileFlag, outputFileMode)
		if err != nil {
			// Logging is secondary to actually starting the service, so just log to stdout and move on.
			ctx.App.Stdout = os.Stdout
		} else {
			defer func() {
				if cErr := outputFile.Close(); rErr == nil && cErr != nil {
					/*
					 * Exit 0 and communicate "success with errors" because:
					 * 1. although we failed to close the output file, we're a cli and the OS will
					 *    close it for us momentarily
					 * 2. there's no standard exit code that specifies cli failure for all commands
					 */
					rErr = cli.WithExitCode(0,
						errors.Errorf("failed to close primary output file: %s",
							launchlib.PrimaryOutputFile))
				}
			}()
			ctx.App.Stdout = outputFile
		}
		return action(ctx)
	}
}

func logErrorAndReturnWithExitCode(ctx cli.Context, err error, exitCode int) cli.ExitCoder {
	// We still want to write the error to stderr if we can't write it to the startup log file.
	_, _ = fmt.Println(ctx.App.Stdout, err)
	return cli.WithExitCode(exitCode, err)
}
