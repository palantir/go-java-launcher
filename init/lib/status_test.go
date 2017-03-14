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

//func TestIsRunning(t *testing.T) {
//	assert.False(t, isRunning(99999))
//
//	myPid := os.Getpid()
//	assert.True(t, isRunning(myPid))
//}
//
//func TestIsRunningByPidFile(t *testing.T) {
//	running, err := IsRunningByPidFile("bogus file")
//	require.Error(t, err)
//	assert.EqualError(t, err, "open bogus file: no such file or directory")
//	assert.Equal(t, running, 3)
//
//	assert.NoError(t, ioutil.WriteFile("pidfile", []byte("99999"), 0644))
//	running, err = IsRunningByPidFile("pidfile")
//	require.NoError(t, err)
//	assert.Equal(t, running, 1)
//
//	assert.NoError(t, ioutil.WriteFile("pidfile", []byte(strconv.Itoa(os.Getpid())), 0644))
//	running, err = IsRunningByPidFile("pidfile")
//	require.NoError(t, err)
//	assert.Equal(t, running, 0)
//
//	assert.NoError(t, os.Remove("pidfile"))
//}
