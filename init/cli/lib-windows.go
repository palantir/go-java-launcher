//go:build windows
// +build windows

package cli

import (
	"fmt"
	"os"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

func isPidRunning(pid int) (bool, *os.Process, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		// If the process is not found, treat it as not running
		fmt.Errorf("isPidRunning: %s", err)
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return false, nil, nil
		}
		return false, nil, err
	}
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
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(proc.Pid))
	if err != nil {
		// If the process is not found, treat it as not running
		fmt.Errorf("isProcRunning: %s", err)
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return false, nil
		}
		return false, err
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false, err
	}
	// If the exit code equals status pending the process has not exited
	// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getexitcodeprocess#remarks
	return windows.NTStatus(exitCode) == windows.STATUS_PENDING, nil
}
