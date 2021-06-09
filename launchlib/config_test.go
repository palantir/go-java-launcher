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
		want PrimaryStaticLauncherConfig
	}{
		{
			name: "java static config",
			data: `
configType: java
configVersion: 1
serviceName: primary
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
			want: PrimaryStaticLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				ServiceName: "primary",
				StaticLauncherConfig: StaticLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "java",
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
		},
		{
			name: "executable static config",
			data: `
configType: executable
configVersion: 1
serviceName: foo
executable: /usr/bin/postgres
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
args:
  - arg1
  - arg2
`,
			want: PrimaryStaticLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				ServiceName: "foo",
				StaticLauncherConfig: StaticLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "executable",
					},
					Env: map[string]string{
						"SOME_ENV_VAR":  "/etc/profile",
						"OTHER_ENV_VAR": "/etc/redhat-release",
					},
					Executable: "/usr/bin/postgres",
					Args:       []string{"arg1", "arg2"},
				},
			},
		},
		{
			name: "with subProcess config",
			data: `
configType: executable
configVersion: 1
serviceName: primary
executable: /usr/bin/postgres
env:
  SOME_ENV_VAR: /etc/profile
  OTHER_ENV_VAR: /etc/redhat-release
args:
  - arg1
  - arg2
subProcesses:
  envoy:
    configType: executable
    executable: /etc/envoy/envoy
    args:
      - arg3
`,
			want: PrimaryStaticLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				ServiceName: "primary",
				StaticLauncherConfig: StaticLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "executable",
					},
					Env: map[string]string{
						"SOME_ENV_VAR":  "/etc/profile",
						"OTHER_ENV_VAR": "/etc/redhat-release",
					},
					Executable: "/usr/bin/postgres",
					Args:       []string{"arg1", "arg2"},
				},
				SubProcesses: map[string]StaticLauncherConfig{
					"envoy": {
						TypedConfig: TypedConfig{
							Type: "executable",
						},
						Executable: "/etc/envoy/envoy",
						Args:       []string{"arg3"},
					},
				},
			},
		},
	} {
		got, _ := parseStaticConfig([]byte(currCase.data))
		assert.Equal(t, currCase.want, got, "Case %d: %s", i, currCase.name)
	}
}

func TestParseCustomConfig(t *testing.T) {
	for i, currCase := range []struct {
		name string
		data string
		want PrimaryCustomLauncherConfig
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
			want: PrimaryCustomLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				CustomLauncherConfig: CustomLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "java",
					},
					Env: map[string]string{
						"SOME_ENV_VAR":  "/etc/profile",
						"OTHER_ENV_VAR": "/etc/redhat-release",
					},
					JvmOpts:                 []string{"jvmOpt1", "jvmOpt2"},
					DisableContainerSupport: false,
				},
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
			want: PrimaryCustomLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				CustomLauncherConfig: CustomLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "java",
					},
					JvmOpts: []string{"jvmOpt1", "jvmOpt2"},
				},
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
			want: PrimaryCustomLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				CustomLauncherConfig: CustomLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "java",
					},
					Env: map[string]string{
						"SOME_ENV_VAR": "{{CWD}}/etc/profile",
					},
					JvmOpts: []string{"jvmOpt1", "jvmOpt2"},
				},
			},
		},
		{
			name: "java custom config with subProcess",
			data: `
configType: java
configVersion: 1
env:
  SOME_ENV_VAR: '{{CWD}}/etc/profile'
jvmOpts:
  - jvmOpt1
  - jvmOpt2
cgroupsV1:
  memory: groupA
  cpuset: groupB
subProcesses:
  envoy:
    configType: executable
    env:
      LOG_LEVEL: info
`,
			want: PrimaryCustomLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				CgroupsV1: map[string]string{
					"memory": "groupA",
					"cpuset": "groupB",
				},
				CustomLauncherConfig: CustomLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "java",
					},
					Env: map[string]string{
						"SOME_ENV_VAR": "{{CWD}}/etc/profile",
					},
					JvmOpts: []string{"jvmOpt1", "jvmOpt2"},
				},
				SubProcesses: map[string]CustomLauncherConfig{
					"envoy": {
						TypedConfig: TypedConfig{
							Type: "executable",
						},
						Env: map[string]string{
							"LOG_LEVEL": "info",
						},
					},
				},
			},
		},
		{
			name: "java custom config with container support disabled",
			data: `
configType: java
configVersion: 1
jvmOpts:
  - jvmOpt1
  - jvmOpt2
dangerousDisableContainerSupport: true
`,
			want: PrimaryCustomLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				CustomLauncherConfig: CustomLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "java",
					},
					JvmOpts:                 []string{"jvmOpt1", "jvmOpt2"},
					DisableContainerSupport: true,
				},
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
			want: PrimaryCustomLauncherConfig{
				VersionedConfig: VersionedConfig{
					Version: 1,
				},
				CustomLauncherConfig: CustomLauncherConfig{
					TypedConfig: TypedConfig{
						Type: "executable",
					},
					Env: map[string]string{
						"SOME_ENV_VAR":  "/etc/profile",
						"OTHER_ENV_VAR": "/etc/redhat-release",
					},
				},
			},
		},
	} {
		got, _ := parseCustomConfig([]byte(currCase.data))
		assert.Equal(t, currCase.want, got, "Case %d: %s", i, currCase.name)
	}
}

func TestParseStaticConfigFailures(t *testing.T) {
	for i, currCase := range []struct {
		name string
		msg  string
		data string
	}{
		{
			name: "bad YAML",
			msg: "Failed to deserialize Static Launcher Config, please check the syntax of your " +
				"configuration file",
			data: `
bad: yaml:
`,
		},
		{
			name: "invalid config type",
			msg:  `Can handle configType\=\{.+\} only, found config`,
			data: `
configType: config
configVersion: 1
serviceName: primary
executable: postgres
`,
		},
		{
			name: "invalid config version",
			msg:  `Can handle configVersion\=\{1\} only, found 2`,
			data: `
configType: executable
configVersion: 2
serviceName: primary
executable: postgres
`,
		},
		{
			name: "invalid subProcess config type",
			msg: "failed to validate subProcess launcher configuration 'incorrect': Can handle " +
				"configType\\=\\{.+\\} only, found config",
			data: `
configType: executable
configVersion: 1
executable: postgres
serviceName: primary
subProcesses:
  incorrect:
    configType: config
`,
		},
		{
			name: "invalid subProcess name",
			msg: "invalid subProcess name '../breakout' in static config: process name '../breakout' " +
				"does not match required pattern '.+'",
			data: `
configType: executable
configVersion: 1
executable: postgres
serviceName: primary
subProcesses:
  ../breakout:
    configType: java
`,
		},
		{
			name: "invalid executable",
			msg:  `Can handle executable\=\{.+\} only, found /bin/rm`,
			data: `
configType: executable
configVersion: 1
executable: /bin/rm
serviceName: primary
args:
  - "-rf"
  - "/"
`,
		},
		{
			name: "missing executable",
			msg:  `Config type \"executable\" requires top-level \"executable:\" value`,
			data: `
configType: executable
serviceName: primary
configVersion: 1
`,
		},
		{
			name: "missing java main class and classpath",
			msg:  `(MainClass|Classpath): zero value`,
			data: `
configType: java
configVersion: 1
serviceName: primary
`,
		},
		{
			name: "missing java main class",
			msg:  `MainClass: zero value`,
			data: `
configType: java
configVersion: 1
serviceName: primary
classpath:
  - thing1
  - thing2
`,
		},
		{
			name: "missing java classpath",
			msg:  `Classpath: zero value`,
			data: `
configType: java
configVersion: 1
serviceName: primary
mainClass: hello.world
`,
		},
		{
			name: "missing service name",
			msg:  "invalid service name '' in static config: process name '' does not match required pattern '.+'",
			data: `
configType: java
configVersion: 1
mainClass: hello.world
classpath:
  - thing1
  - thing2
`,
		},
		{
			name: "invalid service name",
			msg: "invalid service name 'tidle~seps' in static config: process name 'tidle~seps' " +
				"does not match required pattern '.+'",
			data: `
configType: java
configVersion: 1
serviceName: tidle~seps
mainClass: hello.world
classpath:
  - thing1
  - thing2
`,
		},
		{
			name: "subProcess with same service name",
			msg:  "subProcess name 'foo' cannot be the same as ServiceName",
			data: `
configType: java
configVersion: 1
serviceName: foo
mainClass: hello.world
classpath:
  - thing1
  - thing2
subProcesses:
  foo:
    configType: java
`,
		},
	} {
		_, err := parseStaticConfig([]byte(currCase.data))
		assert.NotEqual(t, err, nil, "Case %d: %s had no errors", i, currCase.name)
		assert.Regexp(t, currCase.msg, err.Error(), "Case %d: %s had the wrong error message", i, currCase.name)
	}

}
