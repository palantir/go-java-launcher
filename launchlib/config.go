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

	"gopkg.in/yaml.v2"
)

type JavaConfig struct {
	JavaHome      string            `yaml:"javaHome"`
	MainClass     string            `yaml:"mainClass"`
	JvmOpts       []string 		`yaml:"jvmOpts"`
	Classpath     []string
}

type ShellConfig struct {
	Executable    string            `yaml:"executable"`
}

type StaticLauncherConfig struct {
	ConfigType    string            `yaml:"configType"`
	ConfigVersion int               `yaml:"configVersion"`
	ServiceName   string            `yaml:"serviceName"`
	Env           map[string]string `yaml:"env"`
	Args          []string
	JavaConfig    JavaConfig	`yaml:"java,omitempty"`
	ShellConfig   ShellConfig	`yaml:"shell,omitempty"`
}

type CustomLauncherConfig struct {
	ConfigType    string            `yaml:"configType"`
	ConfigVersion int               `yaml:"configVersion"`
	Env           map[string]string `yaml:"env"`
	JavaConfig    JavaConfig	`yaml:"java,omitempty"`
}

type AllowedConfigValues struct {
	ConfigTypes    	map[string]struct{}
	ConfigVersions 	map[int]struct{}
}

var allowedConfigs = AllowedConfigValues{
	ConfigTypes: 	map[string]struct{}{"java":{},"shell":{}},
	ConfigVersions: map[int]struct{}{2:{}},
}

func (jc JavaConfig) isEmpty() bool {
	if len(jc.JavaHome) > 0 {return false}
	if len(jc.MainClass) > 0 {return false}
	if len(jc.JvmOpts) > 0 {return false}
	if len(jc.Classpath) > 0 {return false}
	return true
}

func (sc ShellConfig) isEmpty() bool {
	if len(sc.Executable) > 0 {return false}
	return true
}

func validateAllowedConfigs(configType string, configVersion int ){
	_, typeOk := allowedConfigs.ConfigTypes[configType]
	_, versionOk := allowedConfigs.ConfigVersions[configVersion]
	if ! typeOk {
		allowedConfigTypes := make([]string, 0, len(allowedConfigs.ConfigTypes))
		for k := range allowedConfigs.ConfigTypes { allowedConfigTypes = append(allowedConfigTypes, k) }

		panic(fmt.Sprintf("Can handle configType=%v only, found %v", allowedConfigTypes, configType))
	}
	if ! versionOk {
		allowedConfigVersions := make([]int, 0, len(allowedConfigs.ConfigVersions))
		for k := range allowedConfigs.ConfigVersions { allowedConfigVersions = append(allowedConfigVersions, k) }

		panic(fmt.Sprintf("Can handle configVersion=%v only, found %v", allowedConfigVersions, configVersion))
	}
}

func validateJavaConfig(javaConfig JavaConfig){
	if javaConfig.isEmpty() {
		panic(fmt.Sprintf("Config type \"java\" requires top-level \"java:\" block"))
	}
}

func validateShellConfig(shellConfig ShellConfig){
	if shellConfig.isEmpty() {
		panic(fmt.Sprintf("Config type \"shell\" requires top-level \"shell:\" block"))
	}
}

func ParseStaticConfig(yamlString []byte) StaticLauncherConfig {
	var config StaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		fmt.Println("Failed to deserialize StaticLauncherConfig, please check the syntax of your configuration file")
		panic(err)
	}
	validateAllowedConfigs(config.ConfigType, config.ConfigVersion)
	if config.ConfigType == "java" { validateJavaConfig(config.JavaConfig) }
	if config.ConfigType == "shell" { validateShellConfig(config.ShellConfig) }
	return config
}

func ParseCustomConfig(yamlString []byte) CustomLauncherConfig {
	var config CustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		fmt.Println("Failed to deserialize CustomLauncherConfig, please check the syntax of your configuration file")
		panic(err)
	}
	validateAllowedConfigs(config.ConfigType, config.ConfigVersion)
	if config.ConfigType == "java" { validateJavaConfig(config.JavaConfig) }
	return config
}
