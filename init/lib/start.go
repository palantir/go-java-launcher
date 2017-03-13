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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
)

func StartCommandWithOutputRedirectionAndPidFile(cmd *exec.Cmd, stdoutFile *os.File, pidFileName string) (int, error) {
	cmd.Stdout = stdoutFile
	cmd.Stderr = stdoutFile
	err := cmd.Start()
	if err != nil {
		return -1, err
	}

	pid := cmd.Process.Pid
	err = ioutil.WriteFile(pidFileName, []byte(strconv.Itoa(pid)), 0644)
	if err != nil {
		return pid, fmt.Errorf("Failed to write pid file: %s", pidFileName)
	}

	return pid, nil
}
