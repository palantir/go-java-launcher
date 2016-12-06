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

func (config *LauncherConfig) validateLauncherConfig() error {
	if _, typeOk := allowedLauncherConfigs.ConfigTypes[config.ConfigType]; !typeOk {
		allowedConfigTypes := make([]string, 0, len(allowedLauncherConfigs.ConfigTypes))
		for k := range allowedLauncherConfigs.ConfigTypes {
			allowedConfigTypes = append(allowedConfigTypes, k)
		}

		return fmt.Errorf("Can handle configType=%v only, found %s", allowedConfigTypes, config.ConfigType)
	}
	if _, versionOk := allowedLauncherConfigs.ConfigVersions[config.ConfigVersion]; !versionOk {
		allowedConfigVersions := make([]int, 0, len(allowedLauncherConfigs.ConfigVersions))
		for k := range allowedLauncherConfigs.ConfigVersions {
			allowedConfigVersions = append(allowedConfigVersions, k)
		}

		return fmt.Errorf("Can handle configVersion=%v only, found %d", allowedConfigVersions, config.ConfigVersion)
	}
	return nil
}

type JavaConfig struct {
	JavaHome  string   `yaml:"javaHome"`
	MainClass string   `yaml:"mainClass"`
	JvmOpts   []string `yaml:"jvmOpts"`
	Classpath []string `yaml:"classpath"`
}

func validateJavaConfig(config JavaConfig) error {
	if config.MainClass == "" {
		return fmt.Errorf("Config type \"java\" requires top-level \"mainClass:\" value")
	}
	if len(config.Classpath) == 0 {
		return fmt.Errorf("Config type \"java\" requires top-level \"classpath:\" array")
	}
	return nil
}

type StaticLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	JavaConfig     `yaml:",inline"`
	ServiceName    string            `yaml:"serviceName"`
	Env            map[string]string `yaml:"env"`
	Executable     string            `yaml:"executable,omitempty"`
	Args           []string          `yaml:"args"`
}

type CustomLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	JvmOpts        []string          `yaml:"jvmOpts"`
	Env            map[string]string `yaml:"env"`
}

func validateExecutableConfig(executable string) error {
	if executable == "" {
		return fmt.Errorf("Config type \"executable\" requires top-level \"executable:\" value")
	}
	if _, executableOk := allowedLauncherConfigs.Executables[path.Base(executable)]; !executableOk {
		allowedExecutables := make([]string, 0, len(allowedLauncherConfigs.Executables))
		for k := range allowedLauncherConfigs.Executables {
			allowedExecutables = append(allowedExecutables, k)
		}

		return fmt.Errorf("Can handle executable=%v only, found %v", allowedExecutables, executable)
	}
	return nil
}

func ParseStaticConfig(yamlString []byte) (StaticLauncherConfig, error) {
	var config StaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return StaticLauncherConfig{}, fmt.Errorf("Failed to deserialize Static Launcher Config, please check the syntax of your configuration file\n")
	}
	if launcherConfigOk := config.LauncherConfig.validateLauncherConfig(); launcherConfigOk != nil {
		return StaticLauncherConfig{}, launcherConfigOk
	}

	if config.ConfigType == "java" {
		if javaConfigOk := validateJavaConfig(config.JavaConfig); javaConfigOk != nil {
			return StaticLauncherConfig{}, javaConfigOk
		}
		config.Executable = "java"
	}

	if executableConfigOk := validateExecutableConfig(config.Executable); executableConfigOk != nil {
		return StaticLauncherConfig{}, executableConfigOk
	}
	return config, nil
}

func ParseCustomConfig(yamlString []byte) (CustomLauncherConfig, error) {
	var config CustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return CustomLauncherConfig{}, fmt.Errorf("Failed to deserialize Custom Launcher Config, please check the syntax of your configuration file\n")
	}
	if launcherConfigOk := config.LauncherConfig.validateLauncherConfig(); launcherConfigOk != nil {
		return CustomLauncherConfig{}, launcherConfigOk
	}
	return config, nil
}
