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

type LauncherConfig struct {
	ConfigType    string `yaml:"configType"`
	ConfigVersion int    `yaml:"configVersion"`
}

type AllowedLauncherConfigValues struct {
	ConfigTypes    	map[string]struct{}
	ConfigVersions 	map[int]struct{}
}

var allowedLauncherConfigs = AllowedLauncherConfigValues{
	ConfigTypes: 	map[string]struct{}{"java":{},"executable":{}},
	ConfigVersions: map[int]struct{}{1:{},2:{}},
}

func (config *LauncherConfig) validateLauncherConfig(){
	_, typeOk := allowedLauncherConfigs.ConfigTypes[config.ConfigType]
	_, versionOk := allowedLauncherConfigs.ConfigVersions[config.ConfigVersion]
	if ! typeOk {
		allowedConfigTypes := make([]string, 0, len(allowedLauncherConfigs.ConfigTypes))
		for k := range allowedLauncherConfigs.ConfigTypes { allowedConfigTypes = append(allowedConfigTypes, k) }

		panic(fmt.Sprintf("Can handle configType=%v only, found %v", allowedConfigTypes, config.ConfigType))
	}
	if ! versionOk {
		allowedConfigVersions := make([]int, 0, len(allowedLauncherConfigs.ConfigVersions))
		for k := range allowedLauncherConfigs.ConfigVersions { allowedConfigVersions = append(allowedConfigVersions, k) }

		panic(fmt.Sprintf("Can handle configVersion=%v only, found %v", allowedConfigVersions, config.ConfigVersion))
	}
}

type JavaConfig struct {
	JavaHome      string            `yaml:"javaHome"`
	MainClass     string            `yaml:"mainClass"`
	JvmOpts       []string 		`yaml:"jvmOpts"`
	Classpath     []string 		`yaml:"classpath"`
}

type StaticLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	ServiceName    string            `yaml:"serviceName"`
	JavaConfig     `yaml:",inline"`
	Env            map[string]string `yaml:"env"`
	Executable     string            `yaml:"executable,omitempty"`
	Args           []string 	 `yaml:"args"`
}

type CustomLauncherConfig struct {
	LauncherConfig `yaml:",inline"`
	JavaConfig     `yaml:",inline"`
	Env            map[string]string `yaml:"env"`
}

func (jc *JavaConfig) isEmpty() bool {
	if len(jc.JavaHome) > 0 {return false}
	if len(jc.MainClass) > 0 {return false}
	if len(jc.JvmOpts) > 0 {return false}
	if len(jc.Classpath) > 0 {return false}
	return true
}

func validateJavaConfig(javaConfig JavaConfig){
	if javaConfig.isEmpty() {
		panic(fmt.Sprintf("Config type \"java\" requires top-level \"java:\" block"))
	}
}

func validateExecutableConfig(executable string){
	if len(executable) <= 0 {
		panic(fmt.Sprintf("Config type \"executable\" requires top-level \"executable:\" value"))
	}
}

func ParseStaticConfig(yamlString []byte) StaticLauncherConfig {
	var config StaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		unmarshalErrPanic("StaticLauncherConfig", err)
	}
	config.LauncherConfig.validateLauncherConfig()
	if config.ConfigType == "java" { validateJavaConfig(config.JavaConfig) }
	if config.ConfigType == "executable" { validateExecutableConfig(config.Executable) }
	return config
}

func ParseCustomConfig(yamlString []byte) CustomLauncherConfig {
	var config CustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		unmarshalErrPanic("CustomLauncherConfig", err)
	}
	config.LauncherConfig.validateLauncherConfig()
	if config.ConfigType == "java" { validateJavaConfig(config.JavaConfig) }
	return config
}

func unmarshalErrPanic(structName string, err error) {
	fmt.Printf("Failed to deserialize %s, please check the syntax of your configuration file\n", structName)
	panic(err)
}
