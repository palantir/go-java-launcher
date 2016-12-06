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
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
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

func toString(inputMap map[string]struct{}) string {
	collection := make([]string, 0, len(inputMap))
	for k := range inputMap {
		collection = append(collection, k)
	}
	return "{" + strings.Join(collection, ", ") + "}"
}

func intMapToStringMap(intMap map[int]struct{}) map[string]struct{} {
	stringMap := make(map[string]struct{})
	for k, v := range intMap {
		stringMap[strconv.Itoa(k)] = v
	}
	return stringMap
}

func (config *LauncherConfig) validateLauncherConfig() error {
	if _, typeOk := allowedLauncherConfigs.ConfigTypes[config.ConfigType]; !typeOk {
		return fmt.Errorf("Can handle configType=%v only, found %s",
			toString(allowedLauncherConfigs.ConfigTypes), config.ConfigType)
	}
	if _, versionOk := allowedLauncherConfigs.ConfigVersions[config.ConfigVersion]; !versionOk {
		return fmt.Errorf("Can handle configVersion=%v only, found %d",
			toString(intMapToStringMap(allowedLauncherConfigs.ConfigVersions)), config.ConfigVersion)
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
		return fmt.Errorf("Can handle executable=%v only, found %v",
			toString(allowedLauncherConfigs.Executables), executable)
	}
	return nil
}

func ParseStaticConfig(yamlString []byte) (StaticLauncherConfig, error) {
	var config StaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return StaticLauncherConfig{}, errors.Wrap(err, "Failed to deserialize Static Launcher Config, please check the syntax of your configuration file\n")
	}
	if launcherConfigErr := config.LauncherConfig.validateLauncherConfig(); launcherConfigErr != nil {
		return StaticLauncherConfig{}, launcherConfigErr
	}

	if config.ConfigType == "java" {
		if javaConfigErr := validateJavaConfig(config.JavaConfig); javaConfigErr != nil {
			return StaticLauncherConfig{}, javaConfigErr
		}
		config.Executable = "java"
	}

	if executableConfigErr := validateExecutableConfig(config.Executable); executableConfigErr != nil {
		return StaticLauncherConfig{}, executableConfigErr
	}
	return config, nil
}

func GetStaticConfigFromFile(staticConfigFile string) (StaticLauncherConfig, error) {
	staticData, err := ioutil.ReadFile(staticConfigFile)
	if err != nil {
		return StaticLauncherConfig{}, errors.Wrap(err, "Failed to read static config file: "+staticConfigFile)
	}
	staticConfig, staticConfigErr := ParseStaticConfig(staticData)
	if staticConfigErr != nil {
		return StaticLauncherConfig{}, staticConfigErr
	}
	return staticConfig, nil
}

func ParseCustomConfig(yamlString []byte) (CustomLauncherConfig, error) {
	var config CustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return CustomLauncherConfig{}, errors.Wrap(err, "Failed to deserialize Custom Launcher Config, please check the syntax of your configuration file\n")
	}
	if launcherConfigErr := config.LauncherConfig.validateLauncherConfig(); launcherConfigErr != nil {
		return CustomLauncherConfig{}, launcherConfigErr
	}
	return config, nil
}

func GetCustomConfigFromFile(customConfigFile string) (CustomLauncherConfig, error) {
	if customData, err := ioutil.ReadFile(customConfigFile); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read custom config file, assuming no custom config:", customConfigFile)
		return CustomLauncherConfig{}, nil
	} else {
		customConfig, customConfigErr := ParseCustomConfig(customData)
		if customConfigErr != nil {
			return CustomLauncherConfig{}, customConfigErr
		}
		return customConfig, nil
	}
}
