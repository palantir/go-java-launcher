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
)

type ErrorResponse struct {
	msg      string
	err      error // underlying error
	exitCode int   // the exit to use when exiting the init program
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%s. Exit code: %d. Underlying error: %s", e.msg, e.exitCode, e.err.Error())
}

type SuccessResponse struct {
	exitCode int // the exit to use when exiting the init program
}

func (e *SuccessResponse) Error() string {
	return fmt.Sprintf("Successful execution. Exit code: %d.", e.exitCode)
}

func respondError(msg string, err error, exitCode int) error {
	return &ErrorResponse{msg, err, exitCode}
}

func respondSuccess(exitCode int) error {
	return &SuccessResponse{exitCode}
}
