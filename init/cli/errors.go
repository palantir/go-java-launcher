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

	"github.com/palantir/go-java-launcher/launchlib"
)

func logErrorAndReturnWithExitCode(err error, exitCode int) cli.ExitCoder {
	// If there's an error logging the error to the primary output file, we still want to write the error to stderr.
	_ = logToPrimaryOutputFile(err.Error())
	return cli.WithExitCode(exitCode, err)
}

func logToPrimaryOutputFile(log string) (rErr error) {
	file, err := os.Create(launchlib.PrimaryOutputFile)
	if err != nil {
		return errors.Wrap(err, "failed to create primary output file")
	}
	defer func() {
		if cErr := file.Close(); rErr == nil && cErr != nil {
			rErr = errors.Wrap(err, "failed to close primary output file")
		}
	}()
	if _, err := file.WriteString(log + "\n"); err != nil {
		return errors.Wrap(err, "failed to write to primary output file")
	}
	return nil
}
