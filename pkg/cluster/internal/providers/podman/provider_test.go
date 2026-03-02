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

package podman

import (
	"testing"
)

func TestProviderString(t *testing.T) {
	t.Parallel()

	p := &provider{}
	if p.String() != "podman" {
		t.Errorf("expected provider string to be 'podman', got %q", p.String())
	}
}

func TestGetHostIPOrDefault(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		hostIP   string
		expected string
	}{
		{
			name:     "empty string defaults to localhost",
			hostIP:   "",
			expected: "127.0.0.1",
		},
		{
			name:     "non-empty string is returned as-is",
			hostIP:   "10.0.0.1",
			expected: "10.0.0.1",
		},
		{
			name:     "ipv6 address is returned as-is",
			hostIP:   "::1",
			expected: "::1",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := getHostIPOrDefault(tc.hostIP)
			if result != tc.expected {
				t.Errorf("getHostIPOrDefault(%q) = %q, want %q", tc.hostIP, result, tc.expected)
			}
		})
	}
}
