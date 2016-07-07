package launchlib

import (
	"gopkg.in/yaml.v2"
	"fmt"
)

type StaticLauncherConfig struct {
	ConfigType    string `yaml:"configType"`
	ConfigVersion int `yaml:"configVersion"`
	ServiceName   string `yaml:"serviceName"`
	MainClass     string `yaml:"mainClass"`
	JavaHome      string `yaml:"javaHome"`
	Classpath     []string
	JvmOpts       []string `yaml:"jvmOpts"`
	Args          []string
}

type CustomLauncherConfig struct {
	ConfigType    string `yaml:"configType"`
	ConfigVersion int `yaml:"configVersion"`
	JvmOpts       []string `yaml:"jvmOpts"`
}

func ParseStaticConfig(yamlString []byte) StaticLauncherConfig {
	var config StaticLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		panic(err)
	}
	if (config.ConfigType != "java") {
		panic(fmt.Sprintf("Can handle configType=java only, found %v", config.ConfigType))
	}
	if (config.ConfigVersion != 1) {
		panic(fmt.Sprintf("Can handle configVersion=1 only, found %v", config.ConfigVersion))
	}
	return config
}

func ParseCustomConfig(yamlString []byte) CustomLauncherConfig {
	var config CustomLauncherConfig
	if err := yaml.Unmarshal(yamlString, &config); err != nil {
		panic(err)
	}
	if (config.ConfigType != "java") {
		panic(fmt.Sprintf("Can handle configType=java only, found %v", config.ConfigType))
	}
	if (config.ConfigVersion != 1) {
		panic(fmt.Sprintf("Can handle configVersion=1 only, found %v", config.ConfigVersion))
	}
	return config
}
