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

package launchlib

import (
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProcessExecutor struct {
	command string
	args    []string
	env     []string
}

func TestGetJavaHome(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	require.NoError(t, os.Setenv("JAVA_HOME", "foo"))

	javaHome := getJavaHome("")
	assert.Equal(t, "foo", javaHome, "JAVA_HOME incorrect")
	javaHome = getJavaHome("explicit javahome")
	assert.Equal(t, "explicit javahome", javaHome, "JAVA_HOME incorrect")

	require.NoError(t, os.Setenv("JAVA_HOME", originalJavaHome))
}

func TestSetCustomEnvironment(t *testing.T) {
	originalEnv := make(map[string]string)
	customEnv := map[string]string{
		"SOME_PATH": "{{CWD}}/full/path",
		"SOME_VAR":  "CUSTOM_VAR",
	}

	env := replaceEnvironmentVariables(merge(originalEnv, customEnv))
	cwd := getWorkingDir()

	if got, ok := env["SOME_PATH"]; ok {
		want := fmt.Sprintf("%s/full/path", cwd)
		assert.Equal(t, want, got, "SOME_PATH environment variable incorrect")
	} else {
		t.Errorf("Expected SOME_PATH to exist in map but it didn't")
	}

	if got, ok := env["SOME_VAR"]; ok {
		assert.Equal(t, "CUSTOM_VAR", got, "SOME_VAR environment variable incorrect")
	} else {
		t.Errorf("Expected CUSTOM_VAR to exist in map, but it didn't")
	}

	m := mockProcessExecutor{}
	args := []string{"arg1", "arg2"}
	execWithChecks("my-command", args, env, &m)

	assert.Equal(t, "my-command", m.command, "Command to be run was incorrect")
	assert.Equal(t, args, m.args)

	startingEnv := os.Environ()
	wantEnv := append(startingEnv, []string{
		fmt.Sprintf("SOME_PATH=%s/full/path", cwd),
		"SOME_VAR=CUSTOM_VAR",
	}...)

	sort.Strings(m.env)
	sort.Strings(wantEnv)
	assert.Equal(t, wantEnv, m.env)
}

func TestUnknownVariablesAreNotExpanded(t *testing.T) {
	originalEnv := make(map[string]string)
	customEnv := map[string]string{
		"SOME_VAR": "{{FOO}}",
	}

	env := replaceEnvironmentVariables(merge(originalEnv, customEnv))
	if got, ok := env["SOME_VAR"]; ok {
		assert.Equal(t, "{{FOO}}", got, "SOME_VAR environment variable incorrect")
	} else {
		t.Errorf("Expected SOME_VAR to exist in map, but it didn't")
	}
}

func TestKeysAreNotExpanded(t *testing.T) {
	originalEnv := make(map[string]string)
	customEnv := map[string]string{
		"{{CWD}}": "Value",
	}

	env := replaceEnvironmentVariables(merge(originalEnv, customEnv))
	if got, ok := env["{{CWD}}"]; ok {
		assert.Equal(t, "Value", got, "%%CWD%% environment variable incorrect")
	} else {
		t.Errorf("Expected %%CWD%% to exist in map and not be expanded, but it didn't")
	}
}

func (m *mockProcessExecutor) Exec(command string, args, env []string) error {
	m.command = command
	m.args = args
	m.env = env
	return nil
}
