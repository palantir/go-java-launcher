// Copyright 2023 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package launchlib

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	memGroupName = "memory"
	memLimitName = "memory.max"
	// DefaultMemoryBytes is the default memory limit in bytes (2GB).
	DefaultMemoryBytes = 2 * 1024 * 1024 * 1024
)

type MemoryLimit interface {
	MemoryLimitInBytes() (uint64, error)
}

var DefaultMemoryLimit = NewCGroupMemoryLimit(os.DirFS("/"))

type CGroupMemoryLimit struct {
	pather CGroupPather
	fs     fs.FS
}

func NewCGroupMemoryLimit(filesystem fs.FS) MemoryLimit {
	return CGroupMemoryLimit{
		pather: NewCGroupV2Pather(filesystem),
		fs:     filesystem,
	}
}

func (c CGroupMemoryLimit) MemoryLimitInBytes() (uint64, error) {
	memoryCGroupPath, err := c.pather.Path(memGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get memory cgroup path")
	}

	memLimitFilepath := filepath.Join(memoryCGroupPath, memLimitName)
	memLimitFile, err := c.fs.Open(convertToFSPath(memLimitFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open memory.max at expected location: %s", memLimitFilepath)
	}
	var closeErr error
	defer func() {
		if cerr := memLimitFile.Close(); cerr != nil && err == nil {
			closeErr = errors.Wrap(cerr, "failed to close memory.max")
		}
	}()

	memLimitBytes, err := io.ReadAll(memLimitFile)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to read memory.max")
	}
	memLimit, err := strconv.Atoi(strings.TrimSpace(string(memLimitBytes)))
	if err != nil {
		return 0, errors.New("unable to convert memory.max value to expected type")
	}

	if closeErr != nil {
		return 0, closeErr
	}

	return uint64(memLimit), nil
}

// MemoryLimiter provides functionality to get memory limits from cgroup v2.
type MemoryLimiter interface {
	MemoryLimit() (uint64, error)
}

// DefaultMemoryLimiter is the default MemoryLimiter that uses the system's cgroup v2.
var DefaultMemoryLimiter = NewMemoryLimiter(DefaultCGroupPather)

type memoryLimiter struct {
	pather CGroupPather
}

// NewMemoryLimiter creates a new MemoryLimiter that uses the given CGroupPather.
func NewMemoryLimiter(pather CGroupPather) MemoryLimiter {
	return &memoryLimiter{pather: pather}
}

// MemoryLimit returns the memory limit in bytes from cgroup v2's memory.max file.
// If the limit cannot be read or parsed, returns DefaultMemoryBytes.
func (m *memoryLimiter) MemoryLimit() (uint64, error) {
	cgroupPath, err := m.pather.Path("memory")
	if err != nil {
		return DefaultMemoryBytes, errors.Wrap(err, "failed to get cgroup path")
	}

	memLimitFile, err := os.Open(filepath.Join(cgroupPath, "memory.max"))
	if err != nil {
		return DefaultMemoryBytes, errors.Wrap(err, "failed to open memory.max")
	}
	var closeErr error
	defer func() {
		if cerr := memLimitFile.Close(); cerr != nil && err == nil {
			closeErr = errors.Wrap(cerr, "failed to close memory.max")
		}
	}()

	content, err := io.ReadAll(memLimitFile)
	if err != nil {
		return DefaultMemoryBytes, errors.Wrap(err, "failed to read memory.max")
	}

	// Parse the memory limit value
	memStr := strings.TrimSpace(string(content))
	memBytes, err := strconv.ParseUint(memStr, 10, 64)
	if err != nil {
		return DefaultMemoryBytes, errors.Wrap(err, "failed to parse memory.max")
	}

	if closeErr != nil {
		return DefaultMemoryBytes, closeErr
	}

	return memBytes, nil
}
