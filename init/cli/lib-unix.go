//go:build !windows
// +build !windows

package cli

import (
	"os"
	"syscall"

	ps "github.com/mitchellh/go-ps"
)

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
