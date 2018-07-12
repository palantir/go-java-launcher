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
	"os/exec"
	"strconv"
	"syscall"

	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/launchlib"
)

const (
	monitorFlag = "--group-monitor"
)

func Exit1WithMessage(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func ParseMonitorArgs(args []string) (*launchlib.ProcessMonitor, error) {
	monitor := &launchlib.ProcessMonitor{}

	var err error
	if monitor.PrimaryPID, err = strconv.Atoi(args[0]); err != nil {
		return nil, errors.Wrapf(err, "error parsing service pid")
	}
	if monitor.ProcessGroupPID, err = strconv.Atoi(args[1]); err != nil {
		return nil, errors.Wrapf(err, "error parsing service group id")
	}
	return monitor, nil
}

func GenerateMonitorArgs(monitor *launchlib.ProcessMonitor) []string {
	return []string{
		monitorFlag,
		strconv.Itoa(monitor.PrimaryPID),
		strconv.Itoa(monitor.ProcessGroupPID),
	}
}

func main() {
	staticConfigFile := "launcher-static.yml"
	customConfigFile := "launcher-custom.yml"
	stdout := os.Stdout

	switch numArgs := len(os.Args); {
	case numArgs == 4 && os.Args[1] == monitorFlag:
		if monitor, err := ParseMonitorArgs(os.Args[2:]); err != nil {
			fmt.Println("error parsing monitor args", err)
			Exit1WithMessage(fmt.Sprintf("Usage: go-java-launcher %s <primary pid> <pgid>", monitorFlag))
		} else if err = monitor.TermProcessGroupOnDeath(); err != nil {
			fmt.Println("error running process monitor", err)
			Exit1WithMessage("process monitor failed")
		}
		return
	case numArgs > 3:
		Exit1WithMessage("Usage: go-java-launcher <path to PrimaryStaticLauncherConfig> " +
			"[<path to PrimaryCustomLauncherConfig>]")
	case numArgs == 2:
		staticConfigFile = os.Args[1]
	case numArgs == 3:
		staticConfigFile = os.Args[1]
		customConfigFile = os.Args[2]
	}

	// Read configuration
	staticConfig, customConfig, err := launchlib.GetConfigsFromFiles(staticConfigFile, customConfigFile, stdout)
	if err != nil {
		fmt.Println("Failed to read config files", err)
		panic(err)
	}

	// Create configured directories
	if err := launchlib.MkDirs(staticConfig.Dirs, stdout); err != nil {
		fmt.Println("Failed to create directories", err)
		panic(err)
	}

	for name, subProcStatic := range staticConfig.SubProcesses {
		if err := launchlib.MkDirs(subProcStatic.Dirs, stdout); err != nil {
			fmt.Println("Failed to create directories for subProcess ", name, err)
			panic(err)
		}
	}

	// Compile command
	cmds, err := launchlib.CompileCmdsFromConfig(&staticConfig, &customConfig, stdout)
	if err != nil {
		fmt.Println("Failed to assemble executable metadata", cmds, err)
		panic(err)
	}

	if len(cmds.SubProcs) != 0 {
		// For this process (referenced as 0), set the process group id to our pid (also referenced as 0), to
		// ensure we are in our own group.
		if err := syscall.Setpgid(0, 0); err != nil {
			fmt.Printf("Unable to create process group for primary with subProcesses")
			panic(err)
		}

		pgid := syscall.Getpgrp()
		monitor := &launchlib.ProcessMonitor{
			PrimaryPID:      os.Getpid(),
			ProcessGroupPID: pgid,
		}
		monitorCmd := exec.Command(os.Args[0], GenerateMonitorArgs(monitor)...)
		monitorCmd.Stdout = os.Stdout
		monitorCmd.Stderr = os.Stderr

		// From this point, if the launcher, or subsequent primary process dies, the process group will be
		// terminated by the process monitor
		fmt.Println("Starting process monitor for service process group ", pgid)
		if err := monitorCmd.Start(); err != nil {
			fmt.Println("Failed to start process monitor for service process group")
			panic(err)
		}

		for name, subProcess := range cmds.SubProcs {
			// Create struct if not present, as to not override previously set SysProcAttr properties
			if subProcess.Cmd.SysProcAttr == nil {
				subProcess.Cmd.SysProcAttr = &syscall.SysProcAttr{}
			}
			// Do not set the pgid of the subProcesses, leaving them in the same process group as this one
			subProcess.Cmd.SysProcAttr.Setpgid = false
			subProcess.Cmd.Stdout = os.Stdout
			subProcess.Cmd.Stderr = os.Stderr

			fmt.Println("Starting subProcesses ", name, subProcess.Cmd.Path)
			if execErr := subProcess.Cmd.Start(); execErr != nil {
				if os.IsNotExist(execErr) {
					fmt.Printf("Executable not found for subProcess %s at: %s\n", name,
						subProcess.Cmd.Path)
				}
				panic(execErr)
			} else {
				fmt.Printf("Started subProcess %s under process pid %d\n", name,
					subProcess.Cmd.Process.Pid)
			}
		}
	}

	execErr := syscall.Exec(cmds.Primary.Cmd.Path, cmds.Primary.Cmd.Args, cmds.Primary.Cmd.Env)
	if execErr != nil {
		if os.IsNotExist(execErr) {
			fmt.Println("Executable not found at:", cmds.Primary.Cmd.Path)
		}
		panic(execErr)
	}
}
