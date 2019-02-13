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

func TestGetJavaHome_usesJAVA_HOMEbydefault(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	require.NoError(t, os.Setenv("JAVA_HOME", "foo"))

	javaHome, javaHomeErr := getJavaHome("")
	assert.Equal(t, "foo", javaHome, "JAVA_HOME incorrect")
	assert.NoError(t, javaHomeErr, "getJavaHome correctly returns nil")
	javaHome, javaHomeErr = getJavaHome("explicit javahome")
	assert.Equal(t, "explicit javahome", javaHome, "JAVA_HOME incorrect")
	assert.NoError(t, javaHomeErr, "getJavaHome correctly returns nil")

	require.NoError(t, os.Setenv("JAVA_HOME", originalJavaHome))
}

func TestGetJavaHome_allowsReadingOtherEnvVar(t *testing.T) {
	original := os.Getenv("SOME_VAR")
	defer func() { require.NoError(t, os.Setenv("SOME_VAR", original)) }()

	require.NoError(t, os.Setenv("SOME_VAR", "foo"))

	javaHome, javaHomeErr := getJavaHome("$SOME_VAR")
	assert.NoError(t, javaHomeErr)
	assert.Equal(t, "foo", javaHome)
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

	args := []string{"arg1", "arg2"}
	cmd, err := createCmd("my-command", args, env)
	assert.NoError(t, err)

	assert.Equal(t, "my-command", cmd.Path, "Command to be run was incorrect")
	assert.Equal(t, args, cmd.Args)

	startingEnv := os.Environ()
	wantEnv := append(startingEnv, []string{
		fmt.Sprintf("SOME_PATH=%s/full/path", cwd),
		"SOME_VAR=CUSTOM_VAR",
	}...)

	sort.Strings(cmd.Env)
	sort.Strings(wantEnv)
	assert.Equal(t, wantEnv, cmd.Env)
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

func TestMkdirChecksDirectorySyntax(t *testing.T) {
	err := MkDirs([]string{"abc/def1"}, os.Stdout)
	assert.NoError(t, err)

	err = MkDirs([]string{"abc"}, os.Stdout)
	assert.NoError(t, err)

	require.NoError(t, os.RemoveAll("abc"))

	badCases := []string{
		"^&*",
		"abc//def",
		"abc/../def",
	}
	for _, dir := range badCases {
		err = MkDirs([]string{dir}, os.Stdout)
		assert.EqualError(t, err, "Cannot create directory with non [A-Za-z0-9] characters: "+dir)
	}
}
