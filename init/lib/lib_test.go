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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var files = []string{LauncherStaticFile, LauncherCustomFile, OutputFile, Pidfile}

func setup(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(file), 0777))
	}

	require.NoError(t, os.Link("testdata/launcher-static-null.yml", LauncherStaticFile))
	require.NoError(t, os.Link("testdata/launcher-custom-null.yml", LauncherCustomFile))
}

func teardown(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.RemoveAll(strings.Split(file, "/")[0]))
	}
}

func writePid(t *testing.T, pid int) {
	require.NoError(t, ioutil.WriteFile(Pidfile, []byte(strconv.Itoa(pid)), 0644))
}

func readPid(t *testing.T) int {
	pidBytes, err := ioutil.ReadFile(Pidfile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(string(pidBytes))
	require.NoError(t, err)
	return pid
}
