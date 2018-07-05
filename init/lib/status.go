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

package lib

import (
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v2"

	"github.com/palantir/go-java-launcher/launchlib"
)

type ServicePids struct {
	PidsByName map[string]int `yaml:"pidsByName" validate:"nonzero"`
}

type ServiceStatusInfo struct {
	RunningProcs   []*os.Process
	NotRunningCmds []NamedCmd
}

type NamedCmd struct {
	Name           string
	Cmd            *exec.Cmd
	OutputFilename string
}

// GetServiceStatus determines the status of the service based on the configuration and the pidfile.
// Returns (ServiceStatusInfo, status, err). Possible values are:
// - (info, 0, nil) if the pidfile exists and can be read and all processes are running
// - (info, 1, <err>) if the pidfile exists and can be read but at least one process is not running
// - (info, 3, <err>) if the pidfile does not exist or cannot be read
// - (nil, 3, <err>) if the config cannot be read
// info contains any running processes recorded in the pidfile (since these are the ones that will need to be stopped to
// stop the service) and the commands that are not running that are defined in the configuration files (since these are
// the ones that will need to be started to start the service).
func GetServiceStatus() (*ServiceStatusInfo, int, error) {
	cmds, err := getConfiguredCommands()
	if err != nil {
		return nil, 3, errors.Wrap(err, "failed to get commands from static and custom configuration files")
	}

	pidfileBytes, err := ioutil.ReadFile(pidfile)
	if err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, cmds}, 3,
			errors.Wrap(err, "failed to read pidfile")
	}
	var servicePids ServicePids
	if err := yaml.Unmarshal(pidfileBytes, &servicePids); err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, cmds}, 3,
			errors.Wrap(err, "failed to deserialize pidfile")
	}
	if err := validator.Validate(servicePids); err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, cmds}, 3,
			errors.Wrap(err, "failed to deserialize pidfile")
	}
	if err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, cmds}, 3,
			errors.Wrap(err, "failed to assemble commands from static and custom configuration files")
	}

	// What processes are running (regardless of if they are configured), and which of the configured processes are not
	// listed in the pidfile or are not running?
	// Look at the pidfile and record what's running.
	runningProcs := make([]*os.Process, 0, len(servicePids.PidsByName))
	for _, pid := range servicePids.PidsByName {
		running, proc := isPidRunning(pid)
		if running {
			runningProcs = append(runningProcs, proc)
		}
	}
	// Then look at the config and record what's not running.
	notRunningCmds := make([]NamedCmd, 0, len(cmds))
	for _, procCmd := range cmds {
		procPid, ok := servicePids.PidsByName[procCmd.Name]
		if !ok {
			notRunningCmds = append(notRunningCmds, procCmd)
		} else {
			running, _ := isPidRunning(procPid)
			if !running {
				notRunningCmds = append(notRunningCmds, procCmd)
			}
		}
	}

	serviceStatusInfo := &ServiceStatusInfo{runningProcs, notRunningCmds}
	if len(notRunningCmds) > 0 {
		return serviceStatusInfo, 1,
			errors.New("pidfile exists and can be read but at least one process is not running")
	}
	return serviceStatusInfo, 0, nil
}

func getConfiguredCommands() (cmds []NamedCmd, rErr error) {
	primaryStdout, err := os.Create(launchlib.PrimaryOutputFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create primary output file: "+launchlib.PrimaryOutputFile)
	}
	defer func() {
		if cErr := primaryStdout.Close(); rErr == nil && cErr != nil {
			rErr = errors.Wrap(err, "failed to close primary output file: "+launchlib.PrimaryOutputFile)
		}
	}()
	staticConfig, customConfig, err := launchlib.GetConfigsFromFiles(launcherStaticFile, launcherCustomFile,
		primaryStdout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read static and custom configuration files")
	}
	serviceCmds, err := launchlib.CompileCmdsFromConfig(&staticConfig, &customConfig, primaryStdout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile commands from static and custom configurations")
	}
	cmds = make([]NamedCmd, 0, 1+len(serviceCmds.SubProcs))
	cmds = append(cmds, NamedCmd{Name: "primary", Cmd: serviceCmds.Primary,
		OutputFilename: serviceCmds.PrimaryOutputFile})
	for name, subProc := range serviceCmds.SubProcs {
		subProcOutputFile, ok := serviceCmds.SubProcsOutputFiles[name]
		if !ok {
			return nil, errors.Wrapf(err,
				"subProcess %s does not have a corresponding output file listed - this is a bug", name)
		}
		cmds = append(cmds, NamedCmd{Name: name, Cmd: subProc, OutputFilename: subProcOutputFile})
	}
	return cmds, nil
}

func isPidRunning(pid int) (bool, *os.Process) {
	// Docs say FindProcess always succeeds on Unix.
	proc, _ := os.FindProcess(pid)
	if isProcRunning(proc) {
		return true, proc
	}
	return false, nil
}

func isProcRunning(proc *os.Process) bool {
	// This is the way to check if a process exists: https://linux.die.net/man/2/kill.
	return proc.Signal(syscall.Signal(0)) == nil
}
