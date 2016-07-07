package main

import (
	"github.com/palantir/go-java-launcher/launchlib"
	"io/ioutil"
	"os"
	"fmt"
)

func LaunchWithConfig(staticConfigFile string, customConfigFile string) {
	staticData, err := ioutil.ReadFile(staticConfigFile)
	if err != nil {
		panic("Failed to read static config file: " + staticConfigFile)
	}
	staticConfig := launchlib.ParseStaticConfig(staticData)

	var customConfig launchlib.CustomLauncherConfig
	if customData, err := ioutil.ReadFile(customConfigFile); err != nil {
		fmt.Println("Failed to read custom config file, assuming no custom config:", customConfigFile)
	} else {
		customConfig = launchlib.ParseCustomConfig(customData)
	}

	launchlib.Launch(&staticConfig, &customConfig)
}

func main() {
	staticConfigFile := "launcher-static.yml"
	customConfigFile := "launcher-custom.yml"

	switch numArgs := len(os.Args); {
	case numArgs > 3:
		panic("Usage: javalauncher [<path to StaticLauncherConfig> [<path to CustomLauncherConfig>]]")
	case numArgs == 2:
		staticConfigFile = os.Args[1]
	case numArgs == 3:
		staticConfigFile = os.Args[1]
		customConfigFile = os.Args[2]
	}

	LaunchWithConfig(staticConfigFile, customConfigFile)
}
