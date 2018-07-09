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
	Pids map[string]int `yaml:"pids" validate:"nonzero"`
}

type CmdWithOutputFile struct {
	Cmd            *exec.Cmd
	OutputFilename string
}

func GetNotRunningCmds() (map[string]CmdWithOutputFile, error) {
	cmds, err := GetConfiguredCommands()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get commands from static and custom configuration files")
	}
	runningProcs, err := GetRunningProcs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine running processes")
	}
	notRunningCmds := make(map[string]CmdWithOutputFile)
	for name, cmd := range cmds {
		if _, ok := runningProcs[name]; !ok {
			notRunningCmds[name] = cmd
		}
	}
	return notRunningCmds, nil
}

func GetRunningProcs() (map[string]*os.Process, error) {
	pidfileBytes, err := ioutil.ReadFile(pidfile)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to read pidfile")
	} else if os.IsNotExist(err) {
		return map[string]*os.Process{}, nil
	}
	var servicePids ServicePids
	if err := yaml.Unmarshal(pidfileBytes, &servicePids); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize pidfile")
	}
	if err := validator.Validate(servicePids); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize pidfile")
	}

	runningProcs := make(map[string]*os.Process)
	for name, pid := range servicePids.Pids {
		running, proc := isPidRunning(pid)
		if running {
			runningProcs[name] = proc
		}
	}
	return runningProcs, nil
}

func GetConfiguredCommands() (cmds map[string]CmdWithOutputFile, rErr error) {
	primaryOutputFile, err := os.Create(launchlib.PrimaryOutputFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create primary output file: "+launchlib.PrimaryOutputFile)
	}
	defer func() {
		if cErr := primaryOutputFile.Close(); rErr == nil && cErr != nil {
			rErr = errors.Wrap(err, "failed to close primary output file: "+launchlib.PrimaryOutputFile)
		}
	}()
	staticConfig, customConfig, err := launchlib.GetConfigsFromFiles(launcherStaticFile, launcherCustomFile,
		primaryOutputFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read static and custom configuration files")
	}
	serviceCmds, err := launchlib.CompileCmdsFromConfig(&staticConfig, &customConfig, primaryOutputFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile commands from static and custom configurations")
	}
	cmds = make(map[string]CmdWithOutputFile)
	cmds["primary"] = CmdWithOutputFile{
		Cmd:            serviceCmds.Primary,
		OutputFilename: serviceCmds.PrimaryOutputFile,
	}
	for name, subProc := range serviceCmds.SubProcs {
		subProcOutputFile, ok := serviceCmds.SubProcsOutputFiles[name]
		if !ok {
			return nil, errors.Wrapf(err,
				"subProcess %s does not have a corresponding output file listed - this is a bug", name)
		}
		cmds[name] = CmdWithOutputFile{Cmd: subProc, OutputFilename: subProcOutputFile}
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
