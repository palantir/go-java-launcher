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

package lib

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
)

// StartCommand starts the given command, outputting to var/log/startup.log and writing the resulting process's PID to
// var/run/service.pid.
func StartCommand(cmd *exec.Cmd, outputFile *os.File) error {
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command")
	}

	if err := ioutil.WriteFile(Pidfile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		return errors.Wrap(err, "failed to write pidfile")
	}

	return nil
}
