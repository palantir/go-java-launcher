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

func isProcRunning(proc *os.Process) (bool, error) {
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(proc.Pid))
	if err != nil {
		return false, err
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false, err
	}

	return exitCode == stillActive, nil
}
