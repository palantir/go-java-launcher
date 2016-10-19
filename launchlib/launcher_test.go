/* Copyright 2015 Palantir Technologies, Inc. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package launchlib

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
)

type mockProcessExecutor struct {
	command string
	args    []string
	env     []string
}

func TestGetJavaHome(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	setEnvOrFail("JAVA_HOME", "foo")

	javaHome := getJavaHome("")
	if javaHome != "foo" {
		t.Error("Expected JAVA_HOME='foo', found", javaHome)
	}
	javaHome = getJavaHome("explicit javahome")
	if javaHome != "explicit javahome" {
		t.Error("Expected JAVA_HOME='explicit javahome', found", javaHome)
	}

	setEnvOrFail("JAVA_HOME", originalJavaHome)
}

func TestSetCustomEnvironment(t *testing.T) {
	originalEnv := make(map[string]string)
	customEnv := map[string]string{
		"SOME_PATH": "%%.CWD%%/full/path",
		"SOME_VAR":  "CUSTOM_VAR",
	}

	fillEnvironmentVariables(originalEnv, customEnv)

	cwd := getWorkingDir()

	if val, ok := originalEnv["SOME_PATH"]; ok {
		expected := fmt.Sprintf("%s/full/path", cwd)
		if val != expected {
			t.Errorf("For SOME_PATH, expected %s, but got %s", expected, val)
		}
	} else {
		t.Errorf("Expected SOME_PATH to exist in map but it didn't")
	}

	if val, ok := originalEnv["SOME_VAR"]; ok {
		if val != "CUSTOM_VAR" {
			t.Errorf("For SOME_VAR, expected %s, but got %s", "CUSTOM_VAR", val)
		}
	} else {
		t.Errorf("Expected CUSTOM_VAR to exist in map, but it didn't")
	}

	m := mockProcessExecutor{}
	args := []string{"arg1", "arg2"}
	execWithChecks("my-command", args, originalEnv, &m)

	if m.command != "my-command" {
		t.Errorf("Expected command to be run was %s, but instead was %s", "my-command", m.command)
	}

	if !reflect.DeepEqual(m.args, args) {
		t.Errorf("Expected incoming args to be %v, but were %v", args, m.args)
	}

	startingEnv := os.Environ()
	expectedEnv := append(startingEnv, []string{
		fmt.Sprintf("SOME_PATH=%s/full/path", cwd),
		"SOME_VAR=CUSTOM_VAR",
	}...)

	sort.Strings(m.env)
	sort.Strings(expectedEnv)
	if !reflect.DeepEqual(m.env, expectedEnv) {
		t.Errorf("Expected custom environment to be %v, but instead was %v", expectedEnv, m.env)
	}

}

func (m *mockProcessExecutor) Exec(command string, args []string, env []string) error {
	m.command = command
	m.args = args
	m.env = env

	return nil
}

func setEnvOrFail(key string, value string) {
	err := os.Setenv(key, value)
	if err != nil {
		panic("Failed to set env var: " + key)
	}
}
