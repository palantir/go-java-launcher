// Copyright 2016 Palantir Technologies, Inc.
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

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"

	"github.com/palantir/go-java-launcher/launchlib"
)

type fileFlags interface {
	Get(name string) int
}

type truncatingFirst struct {
	created map[string]struct{}
}

func (t *truncatingFirst) Get(name string) int {
	if _, ok := t.created[name]; ok {
		return appendOutputFileFlag
	}
	t.created[name] = struct{}{}
	return truncOutputFileFlag
}

func NewTruncatingFirst() fileFlags {
	return &truncatingFirst{
		make(map[string]struct{}),
	}
}

type alwaysAppend struct{}

func (a *alwaysAppend) Get(name string) int {
	return appendOutputFileFlag
}

type FileLoggers struct {
	flags fileFlags
	mode  os.FileMode
}

func (f *FileLoggers) PrimaryLogger() (io.WriteCloser, error) {
	return f.OpenFile(PrimaryOutputFile, f.flags.Get(""))
}

func (f *FileLoggers) SubProcessLogger(name string) launchlib.CreateLogger {
	return func() (io.WriteCloser, error) {
		return f.OpenFile(fmt.Sprintf(SubProcessOutputFileFormat, name), f.flags.Get(name))
	}
}

func (f *FileLoggers) OpenFile(path string, flags int) (*os.File, error) {
	file, err := os.OpenFile(path, flags, f.mode)
	if err != nil {
		return file, errors.Wrapf(err, "could not open logging file '%s'", path)
	}
	return file, nil
}
