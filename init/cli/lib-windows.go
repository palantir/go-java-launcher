//go:build windows
// +build windows

package cli

import (
	"os"
	"syscall"
)

const (
	stillActive = 259
)

func isPidRunning(pid int) (bool, *os.Process, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil, nil
	}
	running, err := isProcRunning(proc)
	if err != nil {
		// If the process is not found, treat it as not running
		if err == syscall.ERROR_PROC_NOT_FOUND {
			return false, nil, nil
		}
		return false, nil, err
	}
	if running {
		return true, proc, nil
	}
	return false, nil, nil
}

func isProcRunning(proc *os.Process) (bool, error) {
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(proc.Pid))
	if err != nil {
		// If the process is not found, treat it as not running
		if err == syscall.ERROR_PROC_NOT_FOUND {
			return false, nil
		}
		return false, err
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false, err
	}

	return exitCode == stillActive, nil
}
