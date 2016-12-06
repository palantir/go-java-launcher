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
configVersion: 2
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
args:
  - arg1
  - arg2
java:
  mainClass: mainClass
  javaHome: javaHome
  classpath:
    - classpath1
    - classpath2
  jvmOpts:
    - jvmOpt1
    - jvmOpt2

`)
	expectedConfig := StaticLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 2,
		Env: map[string]string{
			"SOME_ENV_VAR":  "/etc/profile",
			"OTHER_ENV_VAR": "/etc/redhat-release",
		},
		Args:      []string{"arg1", "arg2"},
		JavaConfig:	JavaConfig{
			MainClass:     "mainClass",
			JavaHome:      "javaHome",
			Classpath: []string{"classpath1", "classpath2"},
			JvmOpts:   []string{"jvmOpt1", "jvmOpt2"},
		}}

	config := ParseStaticConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}
}

func TestParseCustomConfig(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 2
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
java:
  jvmOpts:
    - jvmOpt1
    - jvmOpt2
`)
	expectedConfig := CustomLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 2,
		Env: map[string]string{
			"SOME_ENV_VAR":  "/etc/profile",
			"OTHER_ENV_VAR": "/etc/redhat-release",
		},
		JavaConfig:	JavaConfig{
			JvmOpts: []string{"jvmOpt1", "jvmOpt2"}}}

	config := ParseCustomConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}
}

func TestParseCustomConfigWithoutEnv(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 2
java:
  jvmOpts:
    - jvmOpt1
    - jvmOpt2
`)
	expectedConfig := CustomLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 2,
		JavaConfig:	JavaConfig{
			JvmOpts: []string{"jvmOpt1", "jvmOpt2"}}}

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
configVersion: 2
env:
  SOME_ENV_VAR: '{{CWD}}/etc/profile'
java:
  jvmOpts:
    - jvmOpt1
    - jvmOpt2
`)

	expectedConfig := CustomLauncherConfig{
		ConfigType:    "java",
		ConfigVersion: 2,
		Env: map[string]string{
			"SOME_ENV_VAR": "{{CWD}}/etc/profile",
		},
		JavaConfig:	JavaConfig{
			JvmOpts: []string{"jvmOpt1", "jvmOpt2"}}}

	config := ParseCustomConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}

}
