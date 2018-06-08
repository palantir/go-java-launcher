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
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v2"
	"io"
)

type JavaConfig struct {
	JavaHome  string   `yaml:"javaHome"`
	MainClass string   `yaml:"mainClass" validate:"nonzero"`
	JvmOpts   []string `yaml:"jvmOpts"`
	Classpath []string `yaml:"classpath" validate:"nonzero"`
}

type StaticLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	JavaConfig     `yaml:",inline"`
	ServiceName    string            `yaml:"serviceName"`
	Env            map[string]string `yaml:"env"`
	Executable     string            `yaml:"executable,omitempty"`
	Args           []string          `yaml:"args"`
	Dirs           []string          `yaml:"dirs"`
}

type CustomLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	JvmOpts        []string          `yaml:"jvmOpts"`
	Env            map[string]string `yaml:"env"`
}

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
	Executables:    map[string]struct{}{"java": {}, "postgres": {}, "influxd": {}, "grafana-server": {}, "envoy": {}},
}

func ParseStaticConfig(yamlString []byte) (StaticLauncherConfig, error) {
	var config StaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return StaticLauncherConfig{},
			errors.Wrap(err, "Failed to deserialize Static Launcher Config, please check the syntax of your configuration file")
	}

	if err := config.LauncherConfig.validateLauncherConfig(); err != nil {
		return StaticLauncherConfig{}, err
	}

	if config.ConfigType == "java" {
		config.Executable = "java"
		if err := validator.Validate(config.JavaConfig); err != nil {
			return StaticLauncherConfig{}, err
		}
	}

	if err := validateExecutableConfig(config.Executable); err != nil {
		return StaticLauncherConfig{}, err
	}
	return config, nil
}

func GetStaticConfigFromFile(staticConfigFile string) (StaticLauncherConfig, error) {
	if staticData, err := ioutil.ReadFile(staticConfigFile); err != nil {
		return StaticLauncherConfig{}, errors.Wrap(err, "Failed to read static config file: "+staticConfigFile)
	} else if staticConfig, err := ParseStaticConfig(staticData); err != nil {
		return StaticLauncherConfig{}, err
	} else {
		return staticConfig, nil
	}

}

func ParseCustomConfig(yamlString []byte) (CustomLauncherConfig, error) {
	var config CustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return CustomLauncherConfig{},
			errors.Wrap(err, "Failed to deserialize Custom Launcher Config, please check the syntax of your configuration file")
	}
	if err := config.LauncherConfig.validateLauncherConfig(); err != nil {
		return CustomLauncherConfig{}, err
	}
	return config, nil
}

func GetCustomConfigFromFile(customConfigFile string, stdout io.Writer) (CustomLauncherConfig, error) {
	if customData, err := ioutil.ReadFile(customConfigFile); err != nil {
		fmt.Fprintln(stdout, "Failed to read custom config file, assuming no custom config:", customConfigFile)
		return CustomLauncherConfig{}, nil
	} else if customConfig, err := ParseCustomConfig(customData); err != nil {
		return CustomLauncherConfig{}, err
	} else {
		return customConfig, nil
	}
}

func (config *LauncherConfig) validateLauncherConfig() error {
	if _, ok := allowedLauncherConfigs.ConfigTypes[config.ConfigType]; !ok {
		return fmt.Errorf("Can handle configType=%v only, found %s",
			toString(allowedLauncherConfigs.ConfigTypes), config.ConfigType)
	}
	if _, ok := allowedLauncherConfigs.ConfigVersions[config.ConfigVersion]; !ok {
		return fmt.Errorf("Can handle configVersion=%v only, found %d",
			toString(convertMap(allowedLauncherConfigs.ConfigVersions)), config.ConfigVersion)
	}
	return nil
}

func validateExecutableConfig(executable string) error {
	if executable == "" {
		return errors.New("Config type \"executable\" requires top-level \"executable:\" value")
	}
	if _, ok := allowedLauncherConfigs.Executables[path.Base(executable)]; !ok {
		return fmt.Errorf("Can handle executable=%v only, found %v",
			toString(allowedLauncherConfigs.Executables), executable)
	}
	return nil
}

func toString(inputMap map[string]struct{}) string {
	collection := make([]string, 0, len(inputMap))
	for k := range inputMap {
		collection = append(collection, k)
	}
	return "{" + strings.Join(collection, ", ") + "}"
}

func convertMap(intMap map[int]struct{}) map[string]struct{} {
	stringMap := make(map[string]struct{})
	for k, v := range intMap {
		stringMap[strconv.Itoa(k)] = v
	}
	return stringMap
}
