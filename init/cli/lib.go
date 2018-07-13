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

package cli

import (
	"io/ioutil"
	"os"
	"syscall"

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/palantir/go-java-launcher/launchlib"
)

const (
	outputFileFlag = os.O_WRONLY | os.O_APPEND | os.O_CREATE
	outputFileMode = 0666
)

var (
	launcherStaticFile = "service/bin/launcher-static.yml"
	launcherCustomFile = "var/conf/launcher-custom.yml"
	pidfile            = "var/run/pids.yml"
)

type servicePids map[string]int

type serviceStatus struct {
	notRunningCmds map[string]launchlib.CmdWithOutputFileName
	writtenPids    servicePids
	runningProcs   map[string]*os.Process
}

func getServiceStatus(ctx cli.Context) (*serviceStatus, error) {
	cmds, err := getConfiguredCommands(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get commands from static and custom configuration files")
	}
	writtenPids, runningProcs, err := getPidfileInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine running processes")
	}
	notRunningCmds := make(map[string]launchlib.CmdWithOutputFileName)
	for name, cmd := range cmds {
		if _, ok := runningProcs[name]; !ok {
			notRunningCmds[name] = cmd
		}
	}
	return &serviceStatus{notRunningCmds, writtenPids, runningProcs}, nil
}

func getPidfileInfo() (servicePids, map[string]*os.Process, error) {
	pidfileBytes, err := ioutil.ReadFile(pidfile)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, errors.Wrap(err, "failed to read pidfile")
	} else if os.IsNotExist(err) {
		return servicePids{}, map[string]*os.Process{}, nil
	}
	var servicePids servicePids
	if err := yaml.Unmarshal(pidfileBytes, &servicePids); err != nil {
		return nil, nil, errors.Wrap(err, "failed to deserialize pidfile")
	}
	runningProcs := make(map[string]*os.Process)
	for name, pid := range servicePids {
		if running, proc := isPidRunning(pid); running {
			runningProcs[name] = proc
		}
	}
	return servicePids, runningProcs, nil
}

func getConfiguredCommands(ctx cli.Context) (map[string]launchlib.CmdWithOutputFileName, error) {
	staticConfig, customConfig, err := launchlib.GetConfigsFromFiles(launcherStaticFile, launcherCustomFile,
		ctx.App.Stdout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read static and custom configuration files")
	}
	serviceCmds, err := launchlib.CompileCmdsFromConfig(&staticConfig, &customConfig, ctx.App.Stdout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile commands from static and custom configurations")
	}
	cmds := make(map[string]launchlib.CmdWithOutputFileName)
	cmds[staticConfig.ServiceName] = serviceCmds.Primary
	for name, subProc := range serviceCmds.SubProcs {
		cmds[name] = subProc
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
