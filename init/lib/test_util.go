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

package lib

import (
	"io/ioutil"
	"strconv"
	"os"
	"path/filepath"
	"strings"
)

var files = []string{LauncherStaticFile, LauncherCustomFile, OutputFile, Pidfile}

func setup() {
	Setup("testdata/launcher-static-null.yml", "testdata/launcher-custom-null.yml")
}

func Setup(launcherStaticFilePath string, launcherCustomFilePath string) {
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file), 0777); err != nil {
			panic(err)
		}
	}

	if err := os.Link(launcherStaticFilePath, LauncherStaticFile); err != nil {
		panic(err)
	}
	if err := os.Link(launcherCustomFilePath, LauncherCustomFile); err != nil {
		panic(err)
	}
}

func Teardown() {
	for _, file := range files {
		if err := os.RemoveAll(strings.Split(file, "/")[0]); err != nil {
			panic(err)
		}
	}
}

func WritePid(pid int) {
	if err := ioutil.WriteFile(Pidfile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		panic(err)
	}
}

func ReadPid() int {
	pidBytes, err := ioutil.ReadFile(Pidfile)
	if err != nil {
		panic(err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		panic(err)
	}
	return pid
}
