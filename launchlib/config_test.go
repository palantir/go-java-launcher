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
	"reflect"
	"testing"
)

func TestParseStaticConfig(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 1
mainClass: mainClass
javaHome: javaHome
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
classpath:
  - classpath1
  - classpath2
jvmOpts:
  - jvmOpt1
  - jvmOpt2
args:
  - arg1
  - arg2
`)
	expectedConfig := StaticLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 1,
		MainClass:     "mainClass",
		JavaHome:      "javaHome",
		Env: map[string]string{
			"SOME_ENV_VAR":  "/etc/profile",
			"OTHER_ENV_VAR": "/etc/redhat-release",
		},
		Classpath: []string{"classpath1", "classpath2"},
		JvmOpts:   []string{"jvmOpt1", "jvmOpt2"},
		Args:      []string{"arg1", "arg2"}}

	config := ParseStaticConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}
}

func TestParseCustomConfig(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 1
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
jvmOpts:
  - jvmOpt1
  - jvmOpt2
`)
	expectedConfig := CustomLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 1,
		Env: map[string]string{
			"SOME_ENV_VAR":  "/etc/profile",
			"OTHER_ENV_VAR": "/etc/redhat-release",
		},
		JvmOpts: []string{"jvmOpt1", "jvmOpt2"}}

	config := ParseCustomConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}
}

func TestParseCustomConfigWithoutEnv(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 1
jvmOpts:
  - jvmOpt1
  - jvmOpt2
`)
	expectedConfig := CustomLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 1,
		JvmOpts:       []string{"jvmOpt1", "jvmOpt2"}}

	config := ParseCustomConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}

	if config.Env != nil {
		t.Errorf("Expected environment to be nil, but was %v", config.Env)
	}
}

func TestParseCustomConfigWithEnvPlaceholder(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 1
env:
  SOME_ENV_VAR: '{{CWD}}/etc/profile'
jvmOpts:
  - jvmOpt1
  - jvmOpt2
`)

	expectedConfig := CustomLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 1,
		Env: map[string]string{
			"SOME_ENV_VAR": "{{CWD}}/etc/profile",
		},
		JvmOpts: []string{"jvmOpt1", "jvmOpt2"}}

	config := ParseCustomConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}

}
