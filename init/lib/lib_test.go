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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/palantir/go-java-launcher/launchlib"
)

var files = []string{launchlib.LauncherStaticFile, launchlib.LauncherCustomFile, launchlib.OutputFileFormat, pidfile}

func setupSingleProcess(t *testing.T) {
	setup(t)
	require.NoError(t, os.Link("testdata/launcher-static.yml", launchlib.LauncherStaticFile))
	require.NoError(t, os.Link("testdata/launcher-custom.yml", launchlib.LauncherCustomFile))
}

func setupMultiProcess(t *testing.T) {
	setup(t)
	require.NoError(t, os.Link("testdata/launcher-static-multiprocess.yml", launchlib.LauncherStaticFile))
	require.NoError(t, os.Link("testdata/launcher-custom-multiprocess.yml", launchlib.LauncherCustomFile))
}

func setup(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(file), 0777))
	}
}

func teardown(t *testing.T) {
	for _, file := range files {
		require.NoError(t, os.RemoveAll(strings.Split(file, "/")[0]))
	}
}

func writePidOrFail(t *testing.T, name string, pid int) {
	var servicePids ServicePids
	if pidfileExists() {
		pidfileBytes, err := ioutil.ReadFile(pidfile)
		require.NoError(t, err)
		require.NoError(t, yaml.Unmarshal(pidfileBytes, &servicePids))
	} else {
		servicePids.PidsByName = make(map[string]int)
	}
	servicePids.PidsByName[name] = pid
	servicePidsBytes, err := yaml.Marshal(servicePids)
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(pidfile, servicePidsBytes, 0666))
}

func readPids(t *testing.T) *ServicePids {
	pidfileBytes, err := ioutil.ReadFile(pidfile)
	require.NoError(t, err)
	var servicePids ServicePids
	require.NoError(t, yaml.Unmarshal(pidfileBytes, &servicePids))
	return &servicePids
}
