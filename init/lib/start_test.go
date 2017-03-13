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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStart(t *testing.T) {
	stdoutFile, _ := ioutil.TempFile("", "stdout")
	pidFile, _ := ioutil.TempFile("", "pid")

	// Capture stdout from test context
	originalStdout := os.Stdout
	testStdoutFile, _ := ioutil.TempFile("", "testStdout")
	os.Stdout = testStdoutFile

	cmd := &exec.Cmd{Path: "/bin/ls"}
	pid, err := StartCommandWithOutputRedirectionAndPidFile(cmd, stdoutFile, pidFile.Name())
	assert.NoError(t, err)

	// Assert that output has been written to injected stdoutFile instead of context stdout
	time.Sleep(time.Second) // Wait for forked process to start and print output
	cmdStdout, _ := ioutil.ReadFile(stdoutFile.Name())
	testStdout, _ := ioutil.ReadFile(testStdoutFile.Name())
	assert.Contains(t, string(cmdStdout), "start.go")
	assert.Empty(t, string(testStdout))

	// Assert that pidfile was written
	writtenPid, _ := ioutil.ReadFile(pidFile.Name())
	writtenPidAsInt, _ := strconv.Atoi(string(writtenPid))
	assert.Equal(t, pid, writtenPidAsInt)

	// Reset stdout
	os.Stdout = originalStdout
}
