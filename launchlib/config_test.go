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
	want := StaticLauncherConfig{
		LauncherConfig: LauncherConfig{
			ConfigType:    "java",
			ConfigVersion: 1,
		},
		MainClass: "mainClass",
		JavaHome:  "javaHome",
		Env: map[string]string{
			"SOME_ENV_VAR":  "/etc/profile",
			"OTHER_ENV_VAR": "/etc/redhat-release",
		},
		Classpath: []string{"classpath1", "classpath2"},
		JvmOpts:   []string{"jvmOpt1", "jvmOpt2"},
		Args:      []string{"arg1", "arg2"},
	}

	got := ParseStaticConfig(data)
	assert.Equal(t, want, got)
}

func TestParseCustomConfig(t *testing.T) {
	for i, currCase := range []struct {
		name string
		data string
		want CustomLauncherConfig
	}{
		{
			name: "standard custom config",
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
			name: "custom config without env",
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
			name: "custom config with env placeholder",
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
	} {
		got := ParseCustomConfig([]byte(currCase.data))
		assert.Equal(t, currCase.want, got, "Case %d: %s", i, currCase.name)
	}
}
