//go:build windows
// +build windows

package cli

import (
	"os"
	"syscall"

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

func stopService(ctx cli.Context, procs map[string]*os.Process) error {
	for name, proc := range procs {
		handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, false, uint32(proc.Pid))
		if err != nil {
			// If the process is not found, it's already stopped
			if err == windows.ERROR_INVALID_PARAMETER {
				continue
			}
			return errors.Wrapf(err, "failed to open '%s' process for termination", name)
		}

		defer syscall.CloseHandle(handle)
		if syscall.TerminateProcess(handle, 1) != nil {
			// Windows might return an access denied when attempting to terminate a process that
			// has already finished.
			if err == windows.ERROR_ACCESS_DENIED {
				var exitCode uint32
				// If the exit code equals status pending the process has not exited
				// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getexitcodeprocess#remarks
				if errCode := syscall.GetExitCodeProcess(handle, &exitCode); errCode != nil || windows.NTStatus(exitCode) != windows.STATUS_PENDING {
					// We could not terminate the process due to access denied
					return errors.Wrapf(err, "failed to terminate '%s' process", name)
				}

				// We got denied from terminating a process that already finsished
				continue
			}
			return errors.Wrapf(err, "failed to terminate '%s' process", name)
		}
	}

	if err := waitForServiceToStop(ctx, procs); err != nil {
		return errors.Wrap(err, "failed to stop at least one process")
	}

	return nil
}
