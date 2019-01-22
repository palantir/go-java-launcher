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
	"io"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v2"
)

var (
	processNamePattern = regexp.MustCompile("^[a-z-]+$")
)

type VersionedConfig struct {
	Version int `yaml:"configVersion"`
}

type TypedConfig struct {
	Type string `yaml:"configType"`
}

type JavaConfig struct {
	JavaHome  string   `yaml:"javaHome"`
	MainClass string   `yaml:"mainClass" validate:"nonzero"`
	JvmOpts   []string `yaml:"jvmOpts"`
	Classpath []string `yaml:"classpath" validate:"nonzero"`
}

type StaticLauncherConfig struct {
	TypedConfig `yaml:",inline"`
	JavaConfig  `yaml:",inline"`
	Env         map[string]string `yaml:"env"`
	Executable  string            `yaml:"executable,omitempty"`
	Args        []string          `yaml:"args"`
	Dirs        []string          `yaml:"dirs"`
}

type PrimaryStaticLauncherConfig struct {
	VersionedConfig      `yaml:",inline"`
	ServiceName          string `yaml:"serviceName"`
	StaticLauncherConfig `yaml:",inline"`
	SubProcesses         map[string]StaticLauncherConfig `yaml:"subProcesses"`
}

type CustomLauncherConfig struct {
	TypedConfig `yaml:",inline"`
	JvmOpts     []string          `yaml:"jvmOpts"`
	Env         map[string]string `yaml:"env"`
}

type PrimaryCustomLauncherConfig struct {
	VersionedConfig      `yaml:",inline"`
	CustomLauncherConfig `yaml:",inline"`
	SubProcesses         map[string]CustomLauncherConfig `yaml:"subProcesses"`
}

type AllowedLauncherConfigValues struct {
	ConfigTypes    map[string]struct{}
	ConfigVersions map[int]struct{}
	Executables    map[string]struct{}
}

var allowedLauncherConfigs = AllowedLauncherConfigValues{
	ConfigTypes:    map[string]struct{}{"java": {}, "executable": {}},
	ConfigVersions: map[int]struct{}{1: {}},
	Executables: map[string]struct{}{
		"java":           {},
		"postgres":       {},
		"influxd":        {},
		"grafana-server": {},
		"envoy":          {}},
}

func GetConfigsFromFiles(
	staticConfigFile string, customConfigFile string, stdout io.Writer) (
	PrimaryStaticLauncherConfig, PrimaryCustomLauncherConfig, error) {
	staticConfig, err := getStaticConfigFromFile(staticConfigFile)
	if err != nil {
		return PrimaryStaticLauncherConfig{}, PrimaryCustomLauncherConfig{}, err
	}

	customConfig, err := getCustomConfigFromFile(customConfigFile, stdout)
	if err != nil {
		return PrimaryStaticLauncherConfig{}, PrimaryCustomLauncherConfig{}, err
	}

	// create empty CustomLauncherConfigs for subProcesses not explicitly defined already
	for name, static := range staticConfig.SubProcesses {
		if _, ok := customConfig.SubProcesses[name]; !ok {
			if customConfig.SubProcesses == nil {
				customConfig.SubProcesses = map[string]CustomLauncherConfig{}
			}

			customConfig.SubProcesses[name] = CustomLauncherConfig{
				TypedConfig: TypedConfig{
					Type: static.Type,
				},
			}
		}
	}

	return staticConfig, customConfig, verifyStaticWithCustomConfig(staticConfig, customConfig)
}

func validateSubProcessLimit(numberSubProcesses int) error {
	if numberSubProcesses > 1 {
		return errors.New("only one named subProcesses is currently allowed")
	}
	return nil
}

func validateProcessName(name string) error {
	if !processNamePattern.MatchString(name) {
		return errors.Errorf("process name '%s' does not match required pattern '%s'", name, processNamePattern)
	}
	return nil
}

func parseStaticConfig(yamlString []byte) (PrimaryStaticLauncherConfig, error) {
	var config PrimaryStaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return PrimaryStaticLauncherConfig{},
			errors.Wrap(err, "Failed to deserialize Static Launcher Config, please check the syntax of "+
				"your configuration file")
	}

	if err := config.VersionedConfig.validateVersion(allowedLauncherConfigs.ConfigVersions); err != nil {
		return PrimaryStaticLauncherConfig{}, err
	}

	if err := validateProcessName(config.ServiceName); err != nil {
		return PrimaryStaticLauncherConfig{},
			errors.Wrapf(err, "invalid service name '%s' in static config", config.ServiceName)
	}

	if err := validateStaticConfig(&config.StaticLauncherConfig); err != nil {
		return PrimaryStaticLauncherConfig{}, err
	}

	if err := validateSubProcessLimit(len(config.SubProcesses)); err != nil {
		return PrimaryStaticLauncherConfig{}, err
	}

	for name, subProcess := range config.SubProcesses {
		if err := validateProcessName(name); err != nil {
			return PrimaryStaticLauncherConfig{},
				errors.Wrapf(err, "invalid subProcess name '%s' in static config", name)
		}

		if name == config.ServiceName {
			return PrimaryStaticLauncherConfig{},
				errors.Errorf("subProcess name '%s' cannot be the same as ServiceName", name)
		}

		if err := validateStaticConfig(&subProcess); err != nil {
			return PrimaryStaticLauncherConfig{},
				errors.Wrapf(err, "failed to validate subProcess launcher configuration '%s'", name)
		}
	}
	return config, nil
}

func validateStaticConfig(config *StaticLauncherConfig) error {
	if err := config.TypedConfig.validateType(allowedLauncherConfigs.ConfigTypes); err != nil {
		return err
	}

	if config.Type == "java" {
		config.Executable = "java"
		if err := validator.Validate(config.JavaConfig); err != nil {
			return err
		}
	}

	return validateExecutableConfig(config.Executable)
}

func getStaticConfigFromFile(staticConfigFile string) (PrimaryStaticLauncherConfig, error) {
	if staticData, err := ioutil.ReadFile(staticConfigFile); err != nil {
		return PrimaryStaticLauncherConfig{},
			errors.Wrap(err, "Failed to read static config file: "+staticConfigFile)
	} else if staticConfig, err := parseStaticConfig(staticData); err != nil {
		return PrimaryStaticLauncherConfig{}, err
	} else {
		return staticConfig, nil
	}
}

func verifyStaticWithCustomConfig(staticConfig PrimaryStaticLauncherConfig,
	customConfig PrimaryCustomLauncherConfig) error {
	for name := range customConfig.SubProcesses {
		if _, ok := staticConfig.SubProcesses[name]; !ok {
			return errors.Errorf(
				"custom subProcess config '%s' does not exist in the static config file", name)
		}
	}

	for name, subStatic := range staticConfig.SubProcesses {
		if subCustom, ok := customConfig.SubProcesses[name]; !ok {
			return errors.Errorf(
				"no custom config exists for subProcess '%s' defined in the static config file", name)
		} else if subStatic.Type != subCustom.Type {
			return errors.Errorf(
				"custom config for subProcess '%s' has different type '%s' from static type '%s'",
				name, subCustom.Type, subStatic.Type)
		}
	}
	return nil
}

func parseCustomConfig(yamlString []byte) (PrimaryCustomLauncherConfig, error) {
	var config PrimaryCustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		return PrimaryCustomLauncherConfig{},
			errors.Wrap(err, "Failed to deserialize Custom Launcher Config, please check the syntax of "+
				"your configuration file")
	}

	if err := config.VersionedConfig.validateVersion(allowedLauncherConfigs.ConfigVersions); err != nil {
		return PrimaryCustomLauncherConfig{}, err
	}

	if err := config.TypedConfig.validateType(allowedLauncherConfigs.ConfigTypes); err != nil {
		return PrimaryCustomLauncherConfig{}, err
	}

	if err := validateSubProcessLimit(len(config.SubProcesses)); err != nil {
		return PrimaryCustomLauncherConfig{}, err
	}

	for name, subProcess := range config.SubProcesses {
		if err := validateProcessName(name); err != nil {
			return PrimaryCustomLauncherConfig{}, errors.Wrapf(err, "invalid subProcess name '%s' in "+
				"custom config", name)
		}

		if err := subProcess.TypedConfig.validateType(allowedLauncherConfigs.ConfigTypes); err != nil {
			return PrimaryCustomLauncherConfig{}, errors.Wrapf(err, "invalid launch config in custom "+
				"subProcess config %s", name)
		}
	}
	return config, nil
}

func getCustomConfigFromFile(customConfigFile string, stdout io.Writer) (PrimaryCustomLauncherConfig, error) {
	if customData, err := ioutil.ReadFile(customConfigFile); err != nil {
		fmt.Fprintln(stdout, "Failed to read custom config file, assuming no custom config:",
			customConfigFile)
		return PrimaryCustomLauncherConfig{}, nil
	} else if customConfig, err := parseCustomConfig(customData); err != nil {
		return PrimaryCustomLauncherConfig{}, err
	} else {
		return customConfig, nil
	}
}

func (config *VersionedConfig) validateVersion(allowedVersions map[int]struct{}) error {
	if _, ok := allowedVersions[config.Version]; !ok {
		return fmt.Errorf("Can handle configVersion=%v only, found %d",
			toString(convertMap(allowedVersions)), config.Version)
	}
	return nil
}

func (config *TypedConfig) validateType(allowedTypes map[string]struct{}) error {
	if _, ok := allowedTypes[config.Type]; !ok {
		return fmt.Errorf("Can handle configType=%v only, found %s",
			toString(allowedTypes), config.Type)
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
