package launchlib

import (
	"io"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	cpuMaxName = "cpu.max"
)

var DefaultCGroupV2ProcessorCounter = CGroupV2ProcessorCounter{
	cgroupPaths: NewCGroupV2Pather(defaultFS),
	fs:          defaultFS,
}

type CGroupV2ProcessorCounter struct {
	cgroupPaths CGroupPather
	fs          fs.FS
}

func NewCGroupV2ProcessorCounter(filesystem fs.FS) ProcessorCounter {
	return CGroupV2ProcessorCounter{cgroupPaths: NewCGroupV2Pather(filesystem), fs: filesystem}
}

func (c CGroupV2ProcessorCounter) ProcessorCount() (uint, error) {
	cpuCgroupPath, err := c.cgroupPaths.Path(cpuGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get path to cpu cgroup")
	}

	cpuMaxFilepath := filepath.Join(cpuCgroupPath, cpuMaxName)
	cpuMaxFile, err := c.fs.Open(convertToFSPath(cpuMaxFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open cpu.max at expected location: %s", cpuMaxFilepath)
	}
	defer func() {
		_ = cpuMaxFile.Close()
	}()

	cpuMaxBytes, err := io.ReadAll(cpuMaxFile)
	if err != nil {
		return 0, errors.Wrap(err, "unable to read cpu.max")
	}

	// cpu.max format is "max 100000" or "100000 100000" where the first number is the quota and the second is the period
	fields := strings.Fields(string(cpuMaxBytes))
	if len(fields) != 2 {
		return 0, errors.New("invalid cpu.max format")
	}

	if fields[0] == "max" {
		// No CPU limit set
		return 0, errors.New("no CPU quota set")
	}

	quota, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "unable to parse CPU quota")
	}

	period, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "unable to parse CPU period")
	}

	// The number of CPUs is quota/period
	cpus := quota / period
	return uint(cpus), nil
}
