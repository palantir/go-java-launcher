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
	"io"
	"os"
	"sort"
	"strings"
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

func TestGetJavaHome_defaultsToJavaHomeWhenUndefined(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	require.NoError(t, os.Setenv("JAVA_HOME", "foo"))

	javaHome, javaHomeErr := getJavaHome("$SOME_VAR")
	assert.NoError(t, javaHomeErr)
	assert.Equal(t, "foo", javaHome)

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

func TestFilterHeapSizeArgs(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		heapPercentage  *float64
		wantContains    []string
	}{
		{
			name:         "sets both InitialRAMPercentage and MaxRAMPercentage",
			args:         []string{"-Dfoo=bar"},
			wantContains: []string{"-Dfoo=bar", "-XX:InitialRAMPercentage=75.0", "-XX:MaxRAMPercentage=75.0"},
		},
		{
			name:           "custom heapPercentage sets both InitialRAMPercentage and MaxRAMPercentage",
			args:           []string{"-Dfoo=bar"},
			heapPercentage: toPointer(60.0),
			wantContains:   []string{"-XX:InitialRAMPercentage=60.0", "-XX:MaxRAMPercentage=60.0"},
		},
		{
			name:         "existing RAMPercentage args are preserved",
			args:         []string{"-XX:MaxRAMPercentage=80.0", "-XX:InitialRAMPercentage=50.0"},
			wantContains: []string{"-XX:MaxRAMPercentage=80.0", "-XX:InitialRAMPercentage=50.0"},
		},
		{
			name:         "AlwaysPreTouch is preserved in V1 fallback",
			args:         []string{"-Dfoo=bar", "-XX:+AlwaysPreTouch"},
			wantContains: []string{"-Dfoo=bar", "-XX:+AlwaysPreTouch", "-XX:InitialRAMPercentage=75.0", "-XX:MaxRAMPercentage=75.0"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterHeapSizeArgs(tc.args, tc.heapPercentage)
			for _, want := range tc.wantContains {
				assert.Contains(t, result, want)
			}
		})
	}
}

func TestFilterHeapSizeArgsV2(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		heapPercentage  *float64
		allowHeapShrink bool
		wantXmsPresent  bool
		wantXmxPresent  bool
	}{
		{
			name:            "allowHeapShrink false sets both Xms and Xmx",
			args:            []string{"-Dfoo=bar"},
			allowHeapShrink: false,
			wantXmsPresent:  true,
			wantXmxPresent:  true,
		},
		{
			name:            "allowHeapShrink true omits Xms",
			args:            []string{"-Dfoo=bar"},
			allowHeapShrink: true,
			wantXmsPresent:  false,
			wantXmxPresent:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := filterHeapSizeArgsV2(tc.args, tc.heapPercentage, tc.allowHeapShrink)
			// filterHeapSizeArgsV2 reads cgroup files, which won't exist in test env
			if err != nil {
				t.Skipf("skipping: cgroup files not available: %v", err)
			}
			hasXms := false
			hasXmx := false
			for _, arg := range result {
				if strings.HasPrefix(arg, "-Xms") {
					hasXms = true
				}
				if strings.HasPrefix(arg, "-Xmx") {
					hasXmx = true
				}
			}
			assert.Equal(t, tc.wantXmsPresent, hasXms, "Xms presence")
			assert.Equal(t, tc.wantXmxPresent, hasXmx, "Xmx presence")
		})
	}
}

func TestCompileCmdNativeExecutionMode(t *testing.T) {
	// Create a temporary executable file
	tmpFile, err := os.CreateTemp("", "my-service-native")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	require.NoError(t, tmpFile.Close())
	defer func() { _ = os.Remove(tmpPath) }()

	t.Setenv("CONTAINER", "1")

	staticCfg := StaticLauncherConfig{
		TypedConfig: TypedConfig{Type: "java"}, // value irrelevant for native mode branch
	}

	customCfg := CustomLauncherConfig{
		TypedConfig: TypedConfig{Type: "java"},
		Experimental: ExperimentalLauncherConfig{
			ExecutionMode:             ExecutionModeNative,
			NativeImageExecutablePath: tmpPath,
			NativeImageArguments:      []string{"-XX:MaximumHeapSizePercent=50"},
		},
	}

	cgroups := map[string]string{}
	createLogger := func() (io.WriteCloser, error) { return &NoopClosingWriter{io.Discard}, nil }

	cmd, err := compileCmdFromConfig(&staticCfg, &customCfg, &cgroups, createLogger)
	require.NoError(t, err)

	// Expect command path and arguments
	wantArgs := []string{
		tmpPath,
		"-XX:MaximumHeapSizePercent=50",
	}
	assert.Equal(t, tmpPath, cmd.Path)
	assert.Equal(t, wantArgs, cmd.Args)
}

func TestCompileCmdNativeExecutionModeHeapPercentage(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "nativeExeHP")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	require.NoError(t, tmpFile.Close())
	defer func() { _ = os.Remove(tmpPath) }()

	t.Setenv("CONTAINER", "1")

	heap := 60.1

	staticCfg := StaticLauncherConfig{
		TypedConfig: TypedConfig{Type: "java"},
	}

	customCfg := CustomLauncherConfig{
		TypedConfig:    TypedConfig{Type: "java"},
		HeapPercentage: &heap,
		Experimental: ExperimentalLauncherConfig{
			ExecutionMode:             ExecutionModeNative,
			NativeImageExecutablePath: tmpPath,
			NativeImageArguments:      []string{},
		},
	}

	cgroups := map[string]string{}
	createLogger := func() (io.WriteCloser, error) { return &NoopClosingWriter{io.Discard}, nil }

	cmd, err := compileCmdFromConfig(&staticCfg, &customCfg, &cgroups, createLogger)
	require.NoError(t, err)

	assert.Contains(t, cmd.Args, "-XX:MaximumHeapSizePercent=60")
}
