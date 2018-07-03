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
	NotRunningCmds []launchlib.ProcCmd
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
	procCmds, err := launchlib.CompileCmdsFromConfigFiles()
	if err != nil {
		return nil, 3, errors.Wrap(err, "failed to read static and custom configuration files")
	}

	pidfileBytes, err := ioutil.ReadFile(pidfile)
	if err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, procCmds}, 3, errors.Wrap(err, "failed to read pidfile")
	}
	var servicePids ServicePids
	if err := yaml.Unmarshal(pidfileBytes, &servicePids); err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, procCmds}, 3, errors.Wrap(err, "failed to deserialize pidfile")
	}
	if err := validator.Validate(servicePids); err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, procCmds}, 3, errors.Wrap(err, "failed to deserialize pidfile")
	}
	if err != nil {
		return &ServiceStatusInfo{[]*os.Process{}, procCmds}, 3,
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
	notRunningCmds := make([]launchlib.ProcCmd, 0, len(procCmds))
	for _, procCmd := range procCmds {
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
