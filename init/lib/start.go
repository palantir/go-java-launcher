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

	"github.com/pkg/errors"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v2"
)

func StartService(notRunningCmdsByName map[string]CmdWithOutputFile) (rErr error) {
	pids := make(map[string]int)
	defer func() {
		if err := writePids(pids); err != nil {
			rErr = errors.Wrap(err, "failed to record at least one pid")
		}
	}()
	for name, cmd := range notRunningCmdsByName {
		if err := startCommand(cmd); err != nil {
			return errors.Wrap(err, "failed to start at least one process")
		}
		pids[name] = cmd.Cmd.Process.Pid
	}
	return nil
}

func startCommand(cmd CmdWithOutputFile) (rErr error) {
	stdout, err := os.OpenFile(cmd.OutputFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 266)
	if err != nil {
		return errors.Wrap(err, "failed to open output file: "+cmd.OutputFilename)
	}
	defer func() {
		if cErr := stdout.Close(); rErr == nil && cErr != nil {
			rErr = errors.Wrap(err, "failed to close output file: "+cmd.OutputFilename)
		}
	}()
	cmd.Cmd.Stdout = stdout
	cmd.Cmd.Stderr = stdout
	if err := cmd.Cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command")
	}
	return nil
}

func writePids(pidsByName map[string]int) error {
	var servicePids ServicePids
	pidfileBytes, err := ioutil.ReadFile(pidfile)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to read previous pidfile")
	} else if err != nil && os.IsNotExist(err) {
		servicePids.PidsByName = make(map[string]int)
	} else {
		if err := yaml.Unmarshal(pidfileBytes, &servicePids); err != nil {
			return errors.Wrap(err, "failed to deserialize pidfile")
		}
		if err := validator.Validate(servicePids); err != nil {
			return errors.Wrap(err, "failed to deserialize pidfile")
		}
	}

	for name, pid := range pidsByName {
		servicePids.PidsByName[name] = pid
	}
	servicePidsBytes, err := yaml.Marshal(servicePids)
	if err != nil {
		return errors.Wrap(err, "failed to serialize pidfile")
	}
	if err := ioutil.WriteFile(pidfile, servicePidsBytes, 0666); err != nil {
		return errors.Wrap(err, "failed to write pidfile")
	}

	return nil
}
