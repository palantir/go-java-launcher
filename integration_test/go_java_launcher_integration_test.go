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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/palantir/godel/pkg/products/v2/products"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/palantir/go-java-launcher/launchlib"
)

func TestMainMethod(t *testing.T) {
	output, err := runMainWithArgs(t, "testdata/launcher-static.yml", "testdata/launcher-custom.yml")
	require.NoError(t, err, "failed: %s", output)

	// part of expected output from launcher
	assert.Regexp(t, `Argument list to executable binary: \[.+/bin/java -Xmx4M -Xmx1g -classpath .+/github.com/palantir/go-java-launcher/integration_test/testdata Main arg1\]`, output)
	// expected output of Java program
	assert.Regexp(t, `\nmain method\n`, string(output))
}

func TestPanicsWhenJavaHomeIsNotAFile(t *testing.T) {
	_, err := runMainWithArgs(t, "testdata/launcher-static-bad-java-home.yml", "foo")
	require.Error(t, err, "error: Failed to determine is path is safe to execute: /foo/bar/bin/java")
}

func TestMainMethodWithoutCustomConfig(t *testing.T) {
	output, err := runMainWithArgs(t, "testdata/launcher-static.yml", "foo")
	require.NoError(t, err, "failed: %s", output)

	// part of expected output from launcher
	assert.Regexp(t, `Failed to read custom config file, assuming no custom config: foo`, output)
	assert.Regexp(t, `Argument list to executable binary: \[.+/bin/java -Xmx4M -classpath .+/github.com/palantir/go-java-launcher/integration_test/testdata Main arg1\]`, output)
	// expected output of Java program
	assert.Regexp(t, `\nmain method\n`, string(output))
}

func TestCreatesDirs(t *testing.T) {
	output, err := runMainWithArgs(t, "testdata/launcher-static-with-dirs.yml", "foo")
	require.NoError(t, err, "failed: %s", output)

	dir, err := os.Stat("foo")
	assert.NoError(t, err)
	assert.True(t, dir.IsDir())
	require.NoError(t, os.RemoveAll("foo"))

	dir, err = os.Stat("bar/baz")
	assert.NoError(t, err)
	assert.True(t, dir.IsDir())
	require.NoError(t, os.RemoveAll("bar"))
}

func TestSubProcessesStoppedWhenMainDies(t *testing.T) {
	cmd := mainWithArgs(t, "testdata/launcher-static-multiprocess.yml", "testdata/launcher-custom-multiprocess-long-sub-process.yml")
	children := runMultiProcess(t, cmd)

	assert.NoError(t, cmd.Wait())
	time.Sleep(launchlib.CheckPeriod + 500*time.Millisecond)
	for _, pid := range children {
		assert.False(t, launchlib.IsPidAlive(pid), "child was not killed")
	}
}

func TestSubProcessesParsedMonitorSignals(t *testing.T) {
	cmd := mainWithArgs(t, "testdata/launcher-static-multiprocess.yml", "testdata/launcher-custom-multiprocess-long-sub-process.yml")

	output := &bytes.Buffer{}
	cmd.Stdout = output

	children := runMultiProcess(t, cmd)
	var monitor int
	for cmdLine, pid := range children {
		if strings.Contains(cmdLine, "--group-monitor") {
			monitor = pid
			break
		}
	}

	assert.NotZero(t, monitor, "no monitor pid found")
	require.NoError(t, launchlib.SignalPid(monitor, syscall.SIGPOLL))

	assert.NoError(t, cmd.Wait())

	trapped, err := regexp.Compile("Caught SIGPOLL")
	require.NoError(t, err)
	assert.Len(t, trapped.FindAll(output.Bytes(), -1), 2, "expect two messages that SIGPOLL was caught")
}

func runMainWithArgs(t *testing.T, staticConfigFile, customConfigFile string) (string, error) {
	output, err := mainWithArgs(t, staticConfigFile, customConfigFile).CombinedOutput()
	return string(output), err
}

func mainWithArgs(t *testing.T, staticConfigFile, customConfigFile string) *exec.Cmd {
	cli, err := products.Bin("go-java-launcher")
	require.NoError(t, err)

	return exec.Command(cli, staticConfigFile, customConfigFile)
}

func runMultiProcess(t *testing.T, cmd *exec.Cmd) map[string]int {
	require.NoError(t, cmd.Start())

	// let the launcher create the sub-processes
	time.Sleep(500 * time.Millisecond)
	ppid := cmd.Process.Pid

	command := exec.Command("/bin/ps", "-o", "pid,command", "--no-headers", "--ppid", strconv.Itoa(ppid))
	output, err := command.CombinedOutput()
	require.NoError(t, err)

	children := map[string]int{}
	for _, child := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		parts := strings.SplitN(strings.TrimSpace(child), " ", 2)
		cpid, err := strconv.Atoi(parts[0])
		cmdline := strings.TrimSpace(parts[1])
		require.NoError(t, err)
		// sleep forks into a separate process, we don't want to include it
		if !strings.HasPrefix(cmdline, "sleep") {
			children[strings.TrimSpace(parts[1])] = cpid
		}
	}

	assert.Len(t, children, 2, "there should be one sub-process and one monitor")
	return children
}

func TestMain(m *testing.M) {
	jdkDir := "jdk"
	javaHome, err := filepath.Abs(jdkDir)
	if err != nil {
		log.Fatalf("Failed to calculate absolute path of '%s': %v\n", jdkDir, err)
	}
	if err := os.Setenv("JAVA_HOME", javaHome); err != nil {
		log.Fatalln("Failed to set a mock JAVA_HOME", err)
	}
	os.Exit(m.Run())
}
