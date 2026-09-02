// Copyright 2026 Palantir Technologies, Inc.
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
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyZGCCanaryJvmOpts(t *testing.T) {
	configuredOpts := []string{
		"-XX:+UseG1GC",
		"-XX:+UseNUMA",
		"-XX:MaxGCPauseMillis=500",
		"-XX:-UseZGC",
		"-XX:-ZGenerational",
		"-XX:-ExplicitGCInvokesConcurrent",
		"-Xmx4g",
		"-Dfoo=bar",
	}
	zgcOpts := []string{
		"-Xmx4g",
		"-Dfoo=bar",
		"-XX:+UseZGC",
		"-XX:+ZGenerational",
		"-XX:+ExplicitGCInvokesConcurrent",
	}

	for _, tc := range []struct {
		name        string
		hostname    string
		hostnameErr error
		want        []string
		wantLog     string
	}{
		{
			name:     "matching hostname uses SLS Packaging ZGC profile",
			hostname: "services-mojito-foundry-multipass-1-0",
			want:     zgcOpts,
			wantLog:  `hostname "services-mojito-foundry-multipass-1-0" matches suffix "-1-0"`,
		},
		{
			name:     "non-matching hostname preserves configured options",
			hostname: "services-mojito-foundry-multipass-0-0",
			want:     configuredOpts,
			wantLog:  `hostname "services-mojito-foundry-multipass-0-0" does not match suffix "-1-0"`,
		},
		{
			name:        "hostname failure preserves configured options",
			hostnameErr: errors.New("hostname unavailable"),
			want:        configuredOpts,
			wantLog:     "retaining configured JVM options: hostname unavailable",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			got := applyZGCCanaryJvmOpts(
				configuredOpts,
				"-1-0",
				func() (string, error) { return tc.hostname, tc.hostnameErr },
				&logs,
			)

			assert.Equal(t, tc.want, got)
			assert.Contains(t, logs.String(), tc.wantLog)
		})
	}
}

func TestUseSLSPackagingZGCProfileRemovesOtherCollectorSelectors(t *testing.T) {
	got := useSLSPackagingZGCProfile([]string{
		"-XX:+UseSerialGC",
		"-XX:-UseParallelGC",
		"-XX:+UseParallelOldGC",
		"-XX:+UseParNewGC",
		"-XX:+UseConcMarkSweepGC",
		"-XX:+UseG1GC",
		"-XX:+UseZGC",
		"-XX:+UseShenandoahGC",
		"-XX:+UseEpsilonGC",
	})

	assert.Equal(t, []string{
		"-XX:+UseZGC",
		"-XX:+ZGenerational",
		"-XX:+ExplicitGCInvokesConcurrent",
	}, got)
}
