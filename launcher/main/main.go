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

	"github.com/palantir/go-java-launcher/launchlib"
	"github.com/pkg/errors"
)

const (
	monitorFlag = "--group-monitor"
)

func Exit1WithMessage(message string) {
	_, _ = fmt.Fprintf(os.Stderr, message)
	os.Exit(1)
}

func CreateMonitorFromArgs(primaryPID string, subPIDs []string) (*launchlib.ProcessMonitor, error) {
	monitor := &launchlib.ProcessMonitor{}

	var err error
	if monitor.PrimaryPID, err = strconv.Atoi(primaryPID); err != nil {
		return nil, errors.Wrapf(err, "error parsing service pid")
	}

	for _, pidStr := range subPIDs {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing sub-process pid")
		}
		monitor.SubProcessPIDs = append(monitor.SubProcessPIDs, pid)
	}
	return monitor, nil
}

func GenerateMonitorArgs(monitor *launchlib.ProcessMonitor) []string {
	args := make([]string, 0, len(monitor.SubProcessPIDs)+2)
	args = append(args, monitorFlag, strconv.Itoa(monitor.PrimaryPID))
	for _, pid := range monitor.SubProcessPIDs {
		args = append(args, strconv.Itoa(pid))
	}
	return args
}

func main() {
	staticConfigFile := "launcher-static.yml"
	customConfigFile := "launcher-custom.yml"
	stdout := os.Stdout

	switch numArgs := len(os.Args); {
	case numArgs > 3 && os.Args[1] == monitorFlag:
		monitor, err := CreateMonitorFromArgs(os.Args[2], os.Args[3:])

		if err != nil {
			fmt.Println("error parsing monitor args", err)
			Exit1WithMessage(fmt.Sprintf("Usage: go-java-launcher %s <primary pid> <sub-process pids...>", monitorFlag))
		}

		if err = monitor.Run(); err != nil {
			fmt.Println("error running process monitor", err)
			Exit1WithMessage("process monitor failed")
		}
		return
	case numArgs == 2:
		staticConfigFile = os.Args[1]
	case numArgs == 3:
		staticConfigFile = os.Args[1]
		customConfigFile = os.Args[2]
	default:
		Exit1WithMessage("Usage: go-java-launcher <path to PrimaryStaticLauncherConfig> " +
			"[<path to PrimaryCustomLauncherConfig>]")
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

	// Compile commands
	cmds, err := launchlib.CompileCmdsFromConfig(&staticConfig, &customConfig, launchlib.NewSimpleWriterLogger(os.Stdout))
	if err != nil {
		fmt.Println("Failed to assemble executable metadata", cmds, err)
		panic(err)
	}

	if len(cmds.SubProcesses) != 0 {
		monitor := &launchlib.ProcessMonitor{
			PrimaryPID:     os.Getpid(),
			SubProcessPIDs: nil,
		}
		// From this point, any errors in the launcher will cause all of the created sub-processes to also die,
		// once the main process is exec'ed, this defer will no longer apply, and the external monitor assumes
		// responsibility for killing the sub-processes.
		defer func() {
			if err := monitor.KillSubProcesses(); err != nil {
				// Defer only called if failure complete exec of the primary process, so already panicking
				fmt.Println("error cleaning up sub-processes", err)
			}
		}()

		for name, subProcess := range cmds.SubProcesses {
			subProcess.Stdout = os.Stdout
			subProcess.Stderr = os.Stderr

			fmt.Println("Starting subProcesses ", name, subProcess.Path)
			if execErr := subProcess.Start(); execErr != nil {
				if os.IsNotExist(execErr) {
					fmt.Printf("Executable not found for subProcess %s at: %s\n", name, subProcess.Path)
				}
				panic(execErr)
			}
			monitor.SubProcessPIDs = append(monitor.SubProcessPIDs, subProcess.Process.Pid)
			fmt.Printf("Started subProcess %s under process pid %d\n", name, subProcess.Process.Pid)
		}

		monitorCmd := exec.Command(os.Args[0], GenerateMonitorArgs(monitor)...)
		monitorCmd.Stdout = os.Stdout
		monitorCmd.Stderr = os.Stderr

		fmt.Println("Starting process monitor for service process ", monitor.PrimaryPID)
		if err := monitorCmd.Start(); err != nil {
			fmt.Println("Failed to start process monitor for service process")
			panic(err)
		}
	}

	execErr := syscall.Exec(cmds.Primary.Path, cmds.Primary.Args, cmds.Primary.Env)
	if execErr != nil {
		if os.IsNotExist(execErr) {
			fmt.Println("Executable not found at:", cmds.Primary.Path)
		}
		panic(execErr)
	}
}
