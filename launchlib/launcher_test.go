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

func TestParseMemorySize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
		wantErr  bool
	}{
		{"1g", 1024 * 1024 * 1024, false},
		{"2G", 2 * 1024 * 1024 * 1024, false},
		{"512m", 512 * 1024 * 1024, false},
		{"512M", 512 * 1024 * 1024, false},
		{"1024k", 1024 * 1024, false},
		{"1024K", 1024 * 1024, false},
		{"1073741824", 1073741824, false}, // raw bytes
		{"", 0, true},
		{"invalid", 0, true},
		{"2x", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ParseMemorySize(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestFilterHeapSizeArgsV2_ShrinkableHeap(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		experimental   ExperimentalLauncherConfig
		wantXms        string
		wantXmx        string
		wantNoPreTouch bool
	}{
		{
			name: "shrinkableHeapMaxSize sets Xms to 25% of Xmx",
			args: []string{"-Dfoo=bar"},
			experimental: ExperimentalLauncherConfig{
				ShrinkableHeapMaxSize: "2g",
			},
			wantXms:        fmt.Sprintf("-Xms%d", (2*1024*1024*1024)/4),
			wantXmx:        fmt.Sprintf("-Xmx%d", 2*1024*1024*1024),
			wantNoPreTouch: true,
		},
		{
			name: "shrinkableHeapMaxSize filters out existing AlwaysPreTouch",
			args: []string{"-Dfoo=bar", "-XX:+AlwaysPreTouch"},
			experimental: ExperimentalLauncherConfig{
				ShrinkableHeapMaxSize: "1g",
			},
			wantXms:        fmt.Sprintf("-Xms%d", (1024*1024*1024)/4),
			wantXmx:        fmt.Sprintf("-Xmx%d", 1024*1024*1024),
			wantNoPreTouch: true,
		},
		{
			name: "shrinkableHeapMaxSize filters out existing Xmx/Xms",
			args: []string{"-Xmx4g", "-Xms2g", "-Dfoo=bar"},
			experimental: ExperimentalLauncherConfig{
				ShrinkableHeapMaxSize: "1g",
			},
			wantXms:        fmt.Sprintf("-Xms%d", (1024*1024*1024)/4),
			wantXmx:        fmt.Sprintf("-Xmx%d", 1024*1024*1024),
			wantNoPreTouch: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := filterHeapSizeArgsV2(tc.args, nil, tc.experimental)
			require.NoError(t, err)

			assert.Contains(t, result, tc.wantXms)
			assert.Contains(t, result, tc.wantXmx)
			if tc.wantNoPreTouch {
				assert.Contains(t, result, "-XX:-AlwaysPreTouch")
				assert.NotContains(t, result, "-XX:+AlwaysPreTouch")
				assert.Contains(t, result, "-XX:G1PeriodicGCInterval=600000")
			}
			// Original args should be preserved (except filtered ones)
			assert.Contains(t, result, "-Dfoo=bar")
		})
	}
}
