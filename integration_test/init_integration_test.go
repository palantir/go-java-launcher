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

package integration_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/palantir/godel/pkg/products"
	"github.com/stretchr/testify/assert"

	"github.com/palantir/go-java-launcher/init/lib"
)

func TestInitStatus(t *testing.T) {
	// No valid pidfile
	stdout, stderr, exitCode := runInit("status", "--pidFile", "bogus-file")
	assert.Empty(t, stdout)
	assert.Equal(t, stderr, "Failed to determine whether process is running for pid-file: bogus-file. Exit code: 3. "+
		"Underlying error: open bogus-file: no such file or directory")
	assert.Equal(t, 3, exitCode)

	// Valid pidfile, but corresponding process doesn't exist
	assert.NoError(t, ioutil.WriteFile("pidfile", []byte("99999"), 0644))
	stdout, stderr, exitCode = runInit("status", "--pidFile", "pidfile")
	assert.Equal(t, "Process dead but pidfile exists\n", stdout)
	assert.Empty(t, stderr)
	assert.Equal(t, 1, exitCode)

	// Valid pidfile, process exists
	pid := strconv.Itoa(os.Getpid())
	assert.NoError(t, ioutil.WriteFile("pidfile", []byte(pid), 0644))
	stdout, stderr, exitCode = runInit("status", "--pidFile", "pidfile")
	assert.Equal(t, fmt.Sprintf("Running (%s)\n", pid), stdout)
	assert.Empty(t, stderr)
	assert.Equal(t, 0, exitCode)

	assert.NoError(t, os.Remove("pidfile"))
}

func TestInitStart(t *testing.T) {
	pidFile, _ := ioutil.TempFile("", "pid")
	stdoutFile, _ := ioutil.TempFile("", "stdout")

	// Capture stdout from test context
	originalStdout := os.Stdout
	testStdoutFile, _ := ioutil.TempFile("", "testStdout")
	os.Stdout = testStdoutFile
	defer func() {
		os.Stdout = originalStdout
	}()

	stdout, stderr, exitCode := runInit(
		"start",
		"--launcherStaticFile", "test_resources/launcher-static.yml",
		"--launcherCustomFile", "test_resources/launcher-custom.yml",
		"--pidFile", pidFile.Name(),
		"--outFile", stdoutFile.Name())

	pid, _ := lib.GetPid(pidFile.Name())
	assert.Equal(t, fmt.Sprintf("Started (%d)\n", pid), stdout)
	assert.Empty(t, stderr)
	assert.Equal(t, exitCode, 0)

	time.Sleep(time.Second) // Wait for JVM to start and print output
	out, _ := ioutil.ReadFile(stdoutFile.Name())
	assert.Contains(t, string(out), "Using JAVA_HOME") // command assembly debug output was redirected
	assert.Contains(t, string(out), "main method")     // command output was redirected
}

// Adapted from Stack Overflow: http://stackoverflow.com/questions/10385551/get-exit-code-go
func runInit(args ...string) (stdout string, stderr string, exitCode int) {
	var outbuf, errbuf bytes.Buffer
	cli, err := products.Bin("go-init")
	cmd := exec.Command(cli, args...)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err = cmd.Run()
	stdout = outbuf.String()
	stderr = errbuf.String()

	if err != nil {
		// try to get the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			// This will happen (in OSX) if `name` is not available in $PATH,
			// in this situation, exit code could not be get, and stderr will be
			// empty string very likely, so we use the default fail code, and format err
			// to string and set to stderr
			log.Printf("Could not get exit code for failed program: %v, %v", cli, args)
			if stderr == "" {
				stderr = err.Error()
			}
		}
	} else {
		// success, exitCode should be 0 if go is ok
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}
	return
}
