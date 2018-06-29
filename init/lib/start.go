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

	"github.com/palantir/go-java-launcher/launchlib"
)

func StartService(procCmds []launchlib.ProcCmd) error {
	for _, procCmd := range procCmds {
		if err := startCommand(procCmd); err != nil {
			return errors.Wrap(err, "failed to start at least one process")
		}
		if err := writePid(procCmd.Name, procCmd.Cmd.Process.Pid); err != nil {
			return errors.Wrap(err, "failed to record at least one pid")
		}
	}
	return nil
}

func startCommand(procCmd launchlib.ProcCmd) (rErr error) {
	procCmd.Cmd.Stdout = procCmd.Stdout
	procCmd.Cmd.Stderr = procCmd.Stdout
	if err := procCmd.Cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command")
	}
	return nil
}

func writePid(name string, pid int) error {
	var servicePids ServicePids
	if pidfileExists() {
		pidfileBytes, err := ioutil.ReadFile(pidfile)
		if err != nil {
			return errors.Wrap(err, "failed to read previous pidfile")
		}
		if err := yaml.Unmarshal(pidfileBytes, &servicePids); err != nil {
			return errors.Wrap(err, "failed to deserialize pidfile")
		}
		if err := validator.Validate(servicePids); err != nil {
			return errors.Wrap(err, "failed to deserialize pidfile")
		}
	} else {
		servicePids.PidsByName = make(map[string]int)
	}

	servicePids.PidsByName[name] = pid
	servicePidsBytes, err := yaml.Marshal(servicePids)
	if err != nil {
		return errors.Wrap(err, "failed to serialize pidfile")
	}
	if err := ioutil.WriteFile(pidfile, servicePidsBytes, 0666); err != nil {
		return errors.Wrap(err, "failed to write pidfile")
	}

	return nil
}

func pidfileExists() bool {
	if _, err := os.Stat(pidfile); err != nil {
		// The only piece of information from the error we care about is if the file exists.
		return !os.IsNotExist(err)
	}
	return true
}
