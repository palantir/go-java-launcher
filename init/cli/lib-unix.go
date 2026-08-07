//go:build unix
// +build unix

package cli

import (
	"os"
	"os/exec"
	"syscall"

	ps "github.com/mitchellh/go-ps"
)

func isPidRunning(pid int) (bool, *os.Process, error) {
	// in unix systems os.FindProcess never fails
	proc, _ := os.FindProcess(pid)
	running, err := isProcRunning(proc)
	if err != nil {
		return false, nil, err
	}
	if running {
		return true, proc, nil
	}
	return false, nil, nil
}

func isProcRunning(proc *os.Process) (bool, error) {
	// This is the way to check if a process exists: https://linux.die.net/man/2/kill.
	// On linux, this may respond true if there is a thread running with the same id.
	running := proc.Signal(syscall.Signal(0)) == nil
	if !running {
		return false, nil
	}

	// On linux, iterating over processes will only return running processes. Unfortunately,
	// getting exactly the one pid will also return a thread of the same id.
	procs, err := ps.Processes()
	if err != nil {
		return false, err
	}
	for _, p := range procs {
		if p.Pid() == proc.Pid {
			return true, nil
		}
	}
	return false, nil
}

func setAttrToRunInBackground(cmd *exec.Cmd) {
	// Not necessary for unix
}
