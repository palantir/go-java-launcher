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
	"os"
	"syscall"

	"github.com/palantir/go-java-launcher/launchlib"
)

func Exit1WithMessage(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit((1))
}

func main() {
	staticConfigFile := "launcher-static.yml"
	customConfigFile := "launcher-custom.yml"

	switch numArgs := len(os.Args); {
	case numArgs > 3:
		Exit1WithMessage("Usage: go-java-launcher <path to StaticLauncherConfig> [<path to CustomLauncherConfig>]")
	case numArgs == 2:
		staticConfigFile = os.Args[1]
	case numArgs == 3:
		staticConfigFile = os.Args[1]
		customConfigFile = os.Args[2]
	}

	cmd, err := launchlib.CompileCmdFromConfigFiles(staticConfigFile, customConfigFile)
	if err != nil {
		fmt.Println("Failed to assemble executable metadata", cmd, err)
		panic(err)
	}

	execErr := syscall.Exec(cmd.Path, cmd.Args, cmd.Env)
	if execErr != nil {
		if os.IsNotExist(execErr) {
			fmt.Println("Executable not found at:", cmd.Path)
		}
		panic(execErr)
	}
}
