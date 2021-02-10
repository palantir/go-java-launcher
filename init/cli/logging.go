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
	"io/ioutil"
	"os"
	"strconv"

	"github.com/palantir/go-java-launcher/launchlib"
	"github.com/pkg/errors"
)

type FileFlags interface {
	Get(path string) int
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

func NewTruncatingFirst() FileFlags {
	return &truncatingFirst{
		make(map[string]struct{}),
	}
}

type alwaysAppending struct{}

func (a *alwaysAppending) Get(path string) int {
	return appendOutputFileFlag
}

func NewAlwaysAppending() FileFlags {
	return &alwaysAppending{}
}

type FileLoggers struct {
	flags     FileFlags
	mode      os.FileMode
	openFiles map[string]*os.File
}

func (f *FileLoggers) PrimaryLogger() (io.WriteCloser, error) {
	return f.OpenFile(PrimaryOutputFile)
}

func (f *FileLoggers) SubProcessLogger(name string) launchlib.CreateLogger {
	return func() (io.WriteCloser, error) {
		return f.OpenFile(fmt.Sprintf(SubProcessOutputFileFormat, name))
	}
}

func (f *FileLoggers) OpenFile(path string) (io.WriteCloser, error) {
	if file, ok := f.openFiles[path]; ok {
		return &launchlib.NoopClosingWriter{Writer: file}, nil
	}
	if _, ok := f.flags.(*truncatingFirst); ok {
		backup(path)
	}
	file, err := os.OpenFile(path, f.flags.Get(path), f.mode)
	if err != nil {
		return file, errors.Wrapf(err, "could not open logging file '%s'", path)
	}
	f.openFiles[path] = file
	return &ClosingWriter{WriteCloser: file, openFiles: f.openFiles, path: path}, nil
}

func backup(path string) {
	limit := 5
	os.Remove(path + "." + strconv.Itoa(limit))
	for i := limit; i > 0; i-- {
		os.Rename(path+"."+strconv.Itoa(i-1), path+"."+strconv.Itoa(i))
	}
	os.Rename(path, path+".0")
}

type ClosingWriter struct {
	io.WriteCloser
	openFiles map[string]*os.File
	path      string
}

func (c *ClosingWriter) Close() error {
	delete(c.openFiles, c.path)
	return c.WriteCloser.Close()
}

var devNull = launchlib.NoopClosingWriter{Writer: ioutil.Discard}

type DevNullLoggers struct{}

func (d *DevNullLoggers) PrimaryLogger() (io.WriteCloser, error) {
	return &devNull, nil
}

func (d *DevNullLoggers) SubProcessLogger(name string) launchlib.CreateLogger {
	return d.PrimaryLogger
}
