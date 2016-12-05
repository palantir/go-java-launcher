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

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/palantir/go-java-launcher/launchlib"
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
