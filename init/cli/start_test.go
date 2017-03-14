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
	"testing"

	"github.com/palantir/pkg/cli/flag"
	"github.com/stretchr/testify/assert"
)

func TestInitStart_DefaultParameters(t *testing.T) {
	// Test to prevent accidental changes to parameter default values
	cmd := startCommand()
	assert.Equal(t,
		cmd.Flags,
		[]flag.Flag{
			flag.StringFlag{
				Name:  launcherStaticFileParameter,
				Value: "service/bin/launcher-static.yml",
				Usage: "The location of the LauncherStatic file configuration of the started command"},
			flag.StringFlag{
				Name:  launcherCustomFileParameter,
				Value: "var/conf/launcher-custom.yml",
				Usage: "The location of the LauncherCustom file configuration of the started command"},
			flag.StringFlag{
				Name:  pidfileParameter,
				Value: "var/run/service.pid",
				Usage: "The location of the file storing the process ID of the started command"},
			flag.StringFlag{
				Name:  outFileParameter,
				Value: "var/log/startup.log",
				Usage: "The location of the file to which STDOUT and STDERR of the started command are redirected"},
		})
}
