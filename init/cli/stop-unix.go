//go:build unix
// +build unix

package cli

import (
	"os"
	"strings"
	"syscall"

	"github.com/palantir/pkg/cli"
	"github.com/pkg/errors"
)

func stopService(ctx cli.Context, procs map[string]*os.Process) error {
	for name, proc := range procs {
		if err := proc.Signal(syscall.SIGTERM); err != nil && !strings.Contains(err.Error(),
			"os: process already finished") {
			return errors.Wrapf(err, "failed to stop '%s' process", name)
		}
	}

	if err := waitForServiceToStop(ctx, procs); err != nil {
		return errors.Wrap(err, "failed to stop at least one process")
	}

	return nil
}
