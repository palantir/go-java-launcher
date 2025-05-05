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
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	memGroupName         = "memory"
	cgroupV1MemLimitName = "memory.limit_in_bytes"
	cgroupV2MemLimitName = "memory.max"
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
	isCGroupV2, err := IsCGroupV2(filesystem)
	if err != nil {
		return CGroupMemoryLimit{}
	}

	if isCGroupV2 {
		log.Println("Using cgroupv2 memory limit")
		return CGroupMemoryLimit{
			pather: NewCGroupV2Pather(filesystem),
			fs:     filesystem,
		}
	} else {
		log.Println("Using cgroupv1 memory limit")
		return CGroupMemoryLimit{
			pather: NewCGroupV1Pather(filesystem),
			fs:     filesystem,
		}
	}
}

func (c CGroupMemoryLimit) MemoryLimitInBytes() (uint64, error) {
	memoryCGroupPath, err := c.pather.Path(memGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get memory cgroup path")
	}

	memLimitBytes, err := c.readCGroupV1MemoryLimit(memoryCGroupPath)
	if err != nil {
		log.Println("failed to get cgroupv1 memory limit, trying cgroupv2")
		// If v1 fails, try v2
		memLimitBytes, err = c.readCGroupV2MemoryLimit(memoryCGroupPath)
		if err != nil {
			log.Println("failed to get cgroupv2 memory limit, returning 0")
			return 0, errors.Wrap(err, "failed to read memory limit from both cgroup v1 and v2")
		}
		log.Println("successfully got cgroupv2 memory limit")
	}

	memLimitStr := strings.TrimSpace(string(memLimitBytes))
	if memLimitStr == "max" {
		return 0, errors.New("memory limit is set to max")
	}

	memLimit, err := strconv.Atoi(memLimitStr)
	if err != nil {
		return 0, errors.New("unable to convert memory limit value to expected type")
	}
	return uint64(memLimit), nil
}

func (c CGroupMemoryLimit) readCGroupV1MemoryLimit(memoryCGroupPath string) ([]byte, error) {
	cgroupV1MemLimitFilepath := filepath.Join(memoryCGroupPath, cgroupV1MemLimitName)
	cgroupV1MemLimitFile, err := c.fs.Open(convertToFSPath(cgroupV1MemLimitFilepath))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open memory.limit_in_bytes at expected location: %s", cgroupV1MemLimitFile)
	}
	memLimitBytes, err := io.ReadAll(cgroupV1MemLimitFile)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read memory.limit_in_bytes")
	}
	return memLimitBytes, nil
}

func (c CGroupMemoryLimit) readCGroupV2MemoryLimit(memoryCGroupPath string) ([]byte, error) {
	cgroupV2MemLimitFilepath := filepath.Join(memoryCGroupPath, cgroupV2MemLimitName)
	cgroupV2MemLimitFile, err := c.fs.Open(convertToFSPath(cgroupV2MemLimitFilepath))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open memory.max at expected location: %s", cgroupV2MemLimitFile)
	}
	memLimitBytes, err := io.ReadAll(cgroupV2MemLimitFile)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read memory.max")
	}
	return memLimitBytes, nil
}
