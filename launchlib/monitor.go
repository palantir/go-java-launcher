package launchlib

import (
	"os"
	"syscall"
	"time"

	"github.com/pkg/errors"
)

const (
	CheckPeriod = 5 * time.Second
)

type ProcessMonitor struct {
	ServicePid     int
	ServiceGroupId int
}

func (m *ProcessMonitor) TermProcessGroupOnDeath() error {
	if err := m.verify(); err != nil {
		return err
	}

	tick := time.NewTicker(CheckPeriod)
	alive := true
	for {
		select {
		case <-tick.C:
			alive = m.isAlive()
		}
		if !alive {
			tick.Stop()
			break
		}
	}

	// Service process has died, terminating process group
	if err := syscall.Kill(-m.ServiceGroupId, syscall.SIGTERM); err != nil {
		return errors.Wrapf(err, "unable to set term signal to process group, beware of orphaned secondary services")
	}
	return nil
}

func (m *ProcessMonitor) verify() error {
	if syscall.Getpgrp() != m.ServiceGroupId {
		return errors.Errorf("ProcessMonitor is part of process group '%s' not service process group '%s'. "+
			"ProcessMonitor is expected to only be used by the go-java-launcher itself, under the same process as the"+
			" service", syscall.Getpgrp(), m.ServiceGroupId)
	}

	if m.ServiceGroupId == 1 {
		return errors.New("ProcessMonitor service group given is '1', refusing to monitor services under " +
			"init process group")
	}
	return nil
}

func (m *ProcessMonitor) isAlive() bool {
	process, err := os.FindProcess(m.ServicePid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}
	return true
}
