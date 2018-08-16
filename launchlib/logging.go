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

package launchlib

import (
	"io"
)

type CreateLogger func() (io.WriteCloser, error)

type ServiceLoggers interface {
	PrimaryLogger() (io.WriteCloser, error)
	SubProcessLogger(name string) CreateLogger
}

type SimpleWriterLogger struct {
	writer io.WriteCloser
}

func NewSimpleWriterLogger(writer io.Writer) *SimpleWriterLogger {
	return &SimpleWriterLogger{
		writer: &NoopClosingWriter{writer},
	}
}

func (s *SimpleWriterLogger) PrimaryLogger() (io.WriteCloser, error) {
	return s.writer, nil
}

func (s *SimpleWriterLogger) SubProcessLogger(name string) CreateLogger {
	return func() (io.WriteCloser, error) {
		return s.writer, nil
	}
}

type NoopClosingWriter struct {
	io.Writer
}

func (n *NoopClosingWriter) Close() error {
	// noop
	return nil
}
