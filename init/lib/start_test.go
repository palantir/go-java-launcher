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
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/palantir/go-java-launcher/launchlib"
)

func TestStartService_SingleProcess(t *testing.T) {
	setup(t)
	defer teardown(t)

	cmds := map[string]CmdWithOutputFile{
		"primary": {
			Cmd:            exec.Command("/bin/ls"),
			OutputFilename: launchlib.PrimaryOutputFile,
		},
	}
	assert.NoError(t, StartService(cmds))
	// Wait for process to start up and write output
	time.Sleep(time.Second)

	output, err := ioutil.ReadFile(launchlib.PrimaryOutputFile)
	require.NoError(t, err)
	assert.Contains(t, string(output), "start.go")
	pids := readPids(t).PidsByName
	assert.Equal(t, 1, len(pids))
	assert.Equal(t, cmds["primary"].Cmd.Process.Pid, pids["primary"])
}

func TestStartService_MultiProcess(t *testing.T) {
	setup(t)
	defer teardown(t)

	sidecarOutputFileName := fmt.Sprintf(launchlib.OutputFileFormat, "sidecar-")
	cmds := map[string]CmdWithOutputFile{
		"primary": {
			Cmd:            exec.Command("/bin/ls"),
			OutputFilename: launchlib.PrimaryOutputFile,
		}, "sidecar": {
			Cmd:            exec.Command("/bin/echo", "foo"),
			OutputFilename: sidecarOutputFileName,
		},
	}
	assert.NoError(t, StartService(cmds))

	// Wait for processes to start up and write output
	time.Sleep(time.Second)

	primaryOutput, err := ioutil.ReadFile(launchlib.PrimaryOutputFile)
	require.NoError(t, err)
	assert.Contains(t, string(primaryOutput), "start.go")
	sidecarOutput, err := ioutil.ReadFile(sidecarOutputFileName)
	require.NoError(t, err)
	assert.Contains(t, string(sidecarOutput), "foo")
	pids := readPids(t).PidsByName
	assert.Equal(t, 2, len(pids))
	assert.Equal(t, cmds["primary"].Cmd.Process.Pid, pids["primary"])
	assert.Equal(t, cmds["sidecar"].Cmd.Process.Pid, pids["sidecar"])
}
