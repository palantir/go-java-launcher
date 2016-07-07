package launchlib

import (
	"reflect"
	"testing"
)

func TestParseStaticConfig(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 1
mainClass: mainClass
javaHome: javaHome
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
	expectedConfig := StaticLauncherConfig{
		ConfigType: "java",
		ConfigVersion: 1,
		MainClass: "mainClass",
		JavaHome: "javaHome",
		Classpath: []string { "classpath1", "classpath2" },
		JvmOpts: []string { "jvmOpt1", "jvmOpt2" },
		Args: []string { "arg1", "arg2" }}

	config := ParseStaticConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}
}

func TestParseCustomConfig(t *testing.T) {
	var data = []byte(`
configType: java
configVersion: 1
jvmOpts:
  - jvmOpt1
  - jvmOpt2
`)
	expectedConfig := CustomLauncherConfig{
		ConfigType: "java",
		ConfigVersion: 1,
		JvmOpts: []string { "jvmOpt1", "jvmOpt2" }}

	config := ParseCustomConfig(data)
	if !reflect.DeepEqual(config, expectedConfig) {
		t.Errorf("Expected config %v, found %v", expectedConfig, config)
	}
}
