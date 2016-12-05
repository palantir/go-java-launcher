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
	"os/exec"
	"testing"

	"github.com/palantir/godel/pkg/products"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainMethod(t *testing.T) {
	output, err := runMainWithArgs(t, "test_resources/launcher-static.yml", "test_resources/launcher-custom.yml")
	require.NoError(t, err, "failed: %s", output)

	// part of expected output from launcher
	assert.Regexp(t, `Argument list to Java binary: \[.+/bin/java -Xmx4M -Xmx1g -classpath .+/github.com/palantir/go-java-launcher/integration_test/test_resources Main arg1\]`, output)
	// expected output of Java program
	assert.Regexp(t, `\nmain method\n`, string(output))
}

func TestPanicsWhenJavaHomeIsNotAFile(t *testing.T) {
	_, err := runMainWithArgs(t, "test_resources/launcher-static-bad-java-home.yml", "foo")
	require.Error(t, err, "panic: Failed to determine is path is safe to execute: /foo/bar/bin/java")
}

func TestMainMethodWithoutCustomConfig(t *testing.T) {
	output, err := runMainWithArgs(t, "test_resources/launcher-static.yml", "foo")
	require.NoError(t, err, "failed: %s", output)

	// part of expected output from launcher
	assert.Regexp(t, `Failed to read custom config file, assuming no custom config: foo`, output)
	assert.Regexp(t, `Argument list to Java binary: \[.+/bin/java -Xmx4M -classpath .+/github.com/palantir/go-java-launcher/integration_test/test_resources Main arg1\]`, output)
	// expected output of Java program
	assert.Regexp(t, `\nmain method\n`, string(output))
}

func runMainWithArgs(t *testing.T, staticConfigFile, customConfigFile string) (string, error) {
	cli, err := products.Bin("go-java-launcher")
	require.NoError(t, err)

	cmd := exec.Command(cli, staticConfigFile, customConfigFile)
	outputBytes, err := cmd.CombinedOutput()
	return string(outputBytes), err
}
