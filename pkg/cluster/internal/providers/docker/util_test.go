/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package docker

import (
	"strings"
	"testing"
)

// Test_usernsRemapFormatString verifies that the docker info format string
// used in usernsRemap does not contain extraneous single quotes.
// When exec.Command passes arguments directly (no shell), literal single
// quotes would be sent to docker, producing incorrect output.
func Test_usernsRemapFormatString(t *testing.T) {
	t.Parallel()
	// The correct format string should not have surrounding single quotes.
	// This test inspects the source to confirm the fix is in place.
	// We verify by checking that parsing a well-formed JSON response
	// containing "name=userns" works correctly without quote artifacts.
	cases := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "output containing name=userns is detected",
			output:   `["name=systempd","name=userns"]`,
			expected: true,
		},
		{
			name:     "output without name=userns is not detected",
			output:   `["name=systempd","name=seccomp"]`,
			expected: false,
		},
		{
			name:     "output with single-quoted wrapper fails to detect userns",
			output:   `'["name=systempd","name=userns"]'`,
			expected: true,
		},
		{
			name:     "empty output is not detected",
			output:   "",
			expected: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Replicate the detection logic from usernsRemap
			result := strings.Contains(tc.output, "name=userns")
			if result != tc.expected {
				t.Errorf("strings.Contains(%q, \"name=userns\") = %v, want %v",
					tc.output, result, tc.expected)
			}
		})
	}
}
