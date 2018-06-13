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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStart(t *testing.T) {
	setup()
	defer teardown()

	outputFile, err := os.Create(OutputFile)
	require.NoError(t, err)
	cmd := &exec.Cmd{Path: "/bin/ls"}
	assert.NoError(t, StartCommand(cmd, outputFile))

	// Output was written to OutputFile
	time.Sleep(time.Second) // Wait for forked process to start and print output
	output, err := ioutil.ReadFile(OutputFile)
	require.NoError(t, err)
	assert.Contains(t, string(output), "start.go")
	// Pidfile was written
	readPid()
}
