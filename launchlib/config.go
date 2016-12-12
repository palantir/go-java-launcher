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
	"path"

	"gopkg.in/yaml.v2"
)

type LauncherConfig struct {
	ConfigType    string `yaml:"configType"`
	ConfigVersion int    `yaml:"configVersion"`
}

type AllowedLauncherConfigValues struct {
	ConfigTypes    map[string]struct{}
	ConfigVersions map[int]struct{}
	Executables    map[string]struct{}
}

var allowedLauncherConfigs = AllowedLauncherConfigValues{
	ConfigTypes:    map[string]struct{}{"java": {}, "executable": {}},
	ConfigVersions: map[int]struct{}{1: {}},
	Executables:    map[string]struct{}{"java": {}, "postgres": {}, "influxd": {}, "grafana-server": {}},
}

func (config *LauncherConfig) validateLauncherConfig() {
	if _, typeOk := allowedLauncherConfigs.ConfigTypes[config.ConfigType]; !typeOk {
		allowedConfigTypes := make([]string, 0, len(allowedLauncherConfigs.ConfigTypes))
		for k := range allowedLauncherConfigs.ConfigTypes {
			allowedConfigTypes = append(allowedConfigTypes, k)
		}

		panic(fmt.Sprintf("Can handle configType=%v only, found %s", allowedConfigTypes, config.ConfigType))
	}
	if _, versionOk := allowedLauncherConfigs.ConfigVersions[config.ConfigVersion]; !versionOk {
		allowedConfigVersions := make([]int, 0, len(allowedLauncherConfigs.ConfigVersions))
		for k := range allowedLauncherConfigs.ConfigVersions {
			allowedConfigVersions = append(allowedConfigVersions, k)
		}

		panic(fmt.Sprintf("Can handle configVersion=%v only, found %d", allowedConfigVersions, config.ConfigVersion))
	}
}

type JavaConfig struct {
	JavaHome  string   `yaml:"javaHome"`
	MainClass string   `yaml:"mainClass"`
	JvmOpts   []string `yaml:"jvmOpts"`
	Classpath []string `yaml:"classpath"`
}

type StaticLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	JavaConfig     `yaml:",inline",validate:"nonzero"`
	ServiceName    string            `yaml:"serviceName"`
	Env            map[string]string `yaml:"env"`
	Executable     string            `yaml:"executable,omitempty"`
	Args           []string          `yaml:"args"`
}

type CustomLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	JavaConfig     `yaml:",inline"`
	Env            map[string]string `yaml:"env"`
}

func validateExecutableConfig(executable string) {
	if executable == "" {
		panic(fmt.Sprintf("Config type \"executable\" requires top-level \"executable:\" value"))
	}
	if _, executableOk := allowedLauncherConfigs.Executables[path.Base(executable)]; !executableOk {
		allowedExecutables := make([]string, 0, len(allowedLauncherConfigs.Executables))
		for k := range allowedLauncherConfigs.Executables {
			allowedExecutables = append(allowedExecutables, k)
		}

		panic(fmt.Sprintf("Can handle executable=%v only, found %v", allowedExecutables, executable))
	}
}

func ParseStaticConfig(yamlString []byte) StaticLauncherConfig {
	var config StaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		unmarshalErrPanic("StaticLauncherConfig", err)
	}
	if config.ConfigType == "java" {
		config.Executable = "java"
	}
	validateExecutableConfig(config.Executable)
	config.LauncherConfig.validateLauncherConfig()
	return config
}

func ParseCustomConfig(yamlString []byte) CustomLauncherConfig {
	var config CustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		unmarshalErrPanic("CustomLauncherConfig", err)
	}
	config.LauncherConfig.validateLauncherConfig()
	return config
}

func unmarshalErrPanic(structName string, err error) {
	fmt.Printf("Failed to deserialize %s, please check the syntax of your configuration file\n", structName)
	panic(err)
}
