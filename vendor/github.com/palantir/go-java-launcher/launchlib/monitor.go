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

package launchlib

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
)

const (
	CheckPeriod = 5 * time.Second
)

type ProcessMonitor struct {
	PrimaryPID     int
	SubProcessPIDs []int
}

func (m *ProcessMonitor) Run() error {
	if err := m.verify(); err != nil {
		return err
	}

	m.ForwardSignals()
	return m.TermProcessGroupOnDeath()
}

func (m *ProcessMonitor) ForwardSignals() {
	signals := make(chan os.Signal, 35)
	signal.Notify(signals)

	go func() {
		for {
			select {
			case sign := <-signals:
				// Errors are already printed and there is no where else relevant to return them to.
				_ = SignalPid(m.PrimaryPID, sign)
				_ = m.SignalSubProcesses(sign)
			}
		}
	}()
}

func (m *ProcessMonitor) TermProcessGroupOnDeath() error {
	tick := time.NewTicker(CheckPeriod)
	alive := true
	for {
		select {
		case <-tick.C:
			alive = IsPidAlive(m.PrimaryPID)
		}
		if !alive {
			tick.Stop()
			break
		}
	}

	return m.KillSubProcesses()
}

func (m *ProcessMonitor) KillSubProcesses() error {
	return m.SignalSubProcesses(syscall.SIGTERM)
}

func (m *ProcessMonitor) SignalSubProcesses(sign os.Signal) error {
	// Service process has died, terminating sub-processes
	var errPids []int
	for _, pid := range m.SubProcessPIDs {
		if err := SignalPid(pid, sign); err != nil {
			errPids = append(errPids, pid)
		}
	}

	if len(errPids) > 0 {
		return errors.Errorf("unable to kill sub-processes for pids %v", errPids)
	}
	return nil
}

func (m *ProcessMonitor) verify() error {
	if os.Getppid() != m.PrimaryPID {
		return errors.Errorf("ProcessMonitor is a sub-process of '%d' not service primary process '%d'. "+
			"ProcessMonitor is expected to only be used by the go-java-launcher itself, under the same process as the"+
			" service", os.Getppid(), m.PrimaryPID)
	}

	return nil
}

func IsPidAlive(pid int) bool {
	// This always succeeds on unix systems as it merely creates a process object
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return IsProcessAlive(process)
}

func IsProcessAlive(process *os.Process) bool {
	// Sending a signal of 0 checks the process exists, without actually sending a signal,
	// see https://linux.die.net/man/2/kill
	err := process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}
	return true
}

func SignalPid(pid int, sign os.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil || !IsProcessAlive(process) {
		fmt.Printf("Sub-process %d is dead\n", pid)
		return nil
	}

	if err := process.Signal(sign); err != nil {
		fmt.Println("error signalling sub-process", pid, err, sign)
		return err
	}
	return nil
}
