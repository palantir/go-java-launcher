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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStaticConfig(t *testing.T) {
	for i, currCase := range []struct {
		name string
		data string
		want StaticLauncherConfig
	}{
		{
			name: "java static config",
			data: `
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
`,
			want: StaticLauncherConfig{
				LauncherConfig: LauncherConfig{
					ConfigType:    "java",
					ConfigVersion: 1,
				},
				Env: map[string]string{
					"SOME_ENV_VAR":  "/etc/profile",
					"OTHER_ENV_VAR": "/etc/redhat-release",
				},
				Executable: "java",
				Args:       []string{"arg1", "arg2"},
				JavaConfig: JavaConfig{
					MainClass: "mainClass",
					JavaHome:  "javaHome",
					Classpath: []string{"classpath1", "classpath2"},
					JvmOpts:   []string{"jvmOpt1", "jvmOpt2"},
				},
			},
		},
		{
			name: "executable static config",
			data: `
configType: executable
configVersion: 1
executable: /usr/bin/postgres
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
args:
  - arg1
  - arg2
`,
			want: StaticLauncherConfig{
				LauncherConfig: LauncherConfig{
					ConfigType:    "executable",
					ConfigVersion: 1,
				},
				Env: map[string]string{
					"SOME_ENV_VAR":  "/etc/profile",
					"OTHER_ENV_VAR": "/etc/redhat-release",
				},
				Executable: "/usr/bin/postgres",
				Args:       []string{"arg1", "arg2"},
			},
		},
	} {
		got, _ := ParseStaticConfig([]byte(currCase.data))
		assert.Equal(t, currCase.want, got, "Case %d: %s", i, currCase.name)
	}
}

func TestParseCustomConfig(t *testing.T) {
	for i, currCase := range []struct {
		name string
		data string
		want CustomLauncherConfig
	}{
		{
			name: "java custom config",
			data: `
configType: java
configVersion: 1
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
jvmOpts:
  - jvmOpt1
  - jvmOpt2
`,
			want: CustomLauncherConfig{
				LauncherConfig: LauncherConfig{
					ConfigType:    "java",
					ConfigVersion: 1,
				},
				Env: map[string]string{
					"SOME_ENV_VAR":  "/etc/profile",
					"OTHER_ENV_VAR": "/etc/redhat-release",
				},
				JvmOpts: []string{"jvmOpt1", "jvmOpt2"},
			},
		},
		{
			name: "java custom config without env",
			data: `
configType: java
configVersion: 1
jvmOpts:
  - jvmOpt1
  - jvmOpt2
`,
			want: CustomLauncherConfig{
				LauncherConfig: LauncherConfig{
					ConfigType:    "java",
					ConfigVersion: 1,
				},
				JvmOpts: []string{"jvmOpt1", "jvmOpt2"},
			},
		},
		{
			name: "java custom config with env placeholder v1",
			data: `
configType: java
configVersion: 1
env:
  SOME_ENV_VAR: '{{CWD}}/etc/profile'
jvmOpts:
  - jvmOpt1
  - jvmOpt2
`,
			want: CustomLauncherConfig{
				LauncherConfig: LauncherConfig{
					ConfigType:    "java",
					ConfigVersion: 1,
				},
				Env: map[string]string{
					"SOME_ENV_VAR": "{{CWD}}/etc/profile",
				},
				JvmOpts: []string{"jvmOpt1", "jvmOpt2"},
			},
		},
		{
			name: "executable custom config",
			data: `
configType: executable
configVersion: 1
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
`,
			want: CustomLauncherConfig{
				LauncherConfig: LauncherConfig{
					ConfigType:    "executable",
					ConfigVersion: 1,
				},
				Env: map[string]string{
					"SOME_ENV_VAR":  "/etc/profile",
					"OTHER_ENV_VAR": "/etc/redhat-release",
				},
			},
		},
	} {
		got, _ := ParseCustomConfig([]byte(currCase.data))
		assert.Equal(t, currCase.want, got, "Case %d: %s", i, currCase.name)
	}
}

func TestParseStaticConfigFailures(t *testing.T) {
	for i, currCase := range []struct {
		name string
		data string
	}{
		{
			name: "bad YAML",
			data: `
bad: yaml:
`,
		},
		{
			name: "invalid config type",
			data: `
configType: config
configVersion: 1
executable: postgres
`,
		},
		{
			name: "invalid config version",
			data: `
configType: executable
configVersion: 2
executable: postgres
`,
		},
		{
			name: "invalid executable",
			data: `
configType: executable
configVersion: 1
executable: /bin/rm
args:
  - "-rf"
  - "/"
`,
		},
		{
			name: "missing executable",
			data: `
configType: executable
configVersion: 1
`,
		},
		{
			name: "missing java main class and classpath",
			data: `
configType: java
configVersion: 1
`,
		},
		{
			name: "missing java main class",
			data: `
configType: java
configVersion: 1
classpath:
  - thing1
  - thing2
`,
		},
		{
			name: "missing java classpatg",
			data: `
configType: java
configVersion: 1
mainClass: hello.world
`,
		},
	} {
		_, err := ParseStaticConfig([]byte(currCase.data))
		assert.NotEqual(t, err, nil, "Case %d: %s had no errors", i, currCase.name)
	}
}
