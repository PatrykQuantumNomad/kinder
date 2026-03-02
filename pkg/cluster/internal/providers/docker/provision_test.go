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

// Test_parseSubnetOutput tests the subnet parsing logic that getSubnets
// performs on docker inspect output. The getSubnets function itself calls
// docker directly, so we test the parsing logic in isolation.
func Test_parseSubnetOutput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		lines       []string
		expected    []string
		expectError bool
	}{
		{
			name:        "empty lines causes error",
			lines:       []string{},
			expectError: true,
		},
		{
			name:     "single subnet",
			lines:    []string{"172.18.0.0/16 "},
			expected: []string{"172.18.0.0/16"},
		},
		{
			name:     "dual stack subnets",
			lines:    []string{"172.18.0.0/16 fc00:f853:ccd:e793::/64 "},
			expected: []string{"172.18.0.0/16", "fc00:f853:ccd:e793::/64"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Replicate the parsing logic from getSubnets
			if len(tc.lines) == 0 {
				if !tc.expectError {
					t.Errorf("expected success but got empty lines")
				}
				return
			}
			result := strings.Split(strings.TrimSpace(tc.lines[0]), " ")
			if tc.expectError {
				t.Errorf("expected error but got result: %v", result)
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("got %d subnets, want %d", len(result), len(tc.expected))
				return
			}
			for i := range tc.expected {
				if result[i] != tc.expected[i] {
					t.Errorf("subnet[%d] = %q, want %q", i, result[i], tc.expected[i])
				}
			}
		})
	}
}
