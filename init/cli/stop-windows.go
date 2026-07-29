//go:build windows
// +build windows

package cli

import (
	"os"
	"syscall"

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"
)

func stopService(ctx cli.Context, procs map[string]*os.Process) error {
	for name, proc := range procs {
		handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, false, uint32(proc.Pid))
		if err != nil {
			// If the process is not found, it's already stopped
			if err == syscall.ERROR_PROC_NOT_FOUND {
				continue
			}
			return errors.Wrapf(err, "failed to open '%s' process for termination", name)
		}
		defer syscall.CloseHandle(handle)

		if err := syscall.TerminateProcess(handle, 1); err != nil {
			return errors.Wrapf(err, "failed to terminate '%s' process", name)
		}
	}

	if err := waitForServiceToStop(ctx, procs); err != nil {
		return errors.Wrap(err, "failed to stop at least one process")
	}

	return nil
}
