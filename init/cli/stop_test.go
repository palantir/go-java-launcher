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

// To prevent accidental changes to parameter default values
func TestInitStop_DefaultParameters(t *testing.T) {
	cmd := stopCommand()
	assert.Equal(t,
		cmd.Flags,
		[]flag.Flag(nil))
}
