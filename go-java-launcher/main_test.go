/* Copyright 2015 Palantir Technologies, Inc. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"testing"
)

func TestMainMethod(t *testing.T) {
	LaunchWithConfig("test_resources/launcher-static.yml", "test_resources/launcher-custom.yml")
}

func TestPanicsWhenJavaHomeIsNotAFile(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected launcher to fail when JAVA_HOME is bad")
		}
	}()
	LaunchWithConfig("test_resources/launcher-static-bad-java-home.yml", "foo")
}

func TestMainMethodWithoutCustomConfig(t *testing.T) {
	LaunchWithConfig("test_resources/launcher-static.yml", "foo")
}
