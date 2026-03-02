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

package nerdctl

import (
	"fmt"
	"testing"

	"sigs.k8s.io/kind/pkg/exec"
)

func TestIsPoolOverlapError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "non-RunError returns false",
			err:      fmt.Errorf("some random error"),
			expected: false,
		},
		{
			name: "RunError with pool overlap prefix returns true",
			err: &exec.RunError{
				Command: []string{"nerdctl", "network", "create"},
				Output:  []byte("Error response from daemon: Pool overlaps with other one on this address space"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with overlapping networks message returns true",
			err: &exec.RunError{
				Command: []string{"nerdctl", "network", "create"},
				Output:  []byte("some prefix: networks have overlapping IPv4"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with unrelated message returns false",
			err: &exec.RunError{
				Command: []string{"nerdctl", "network", "create"},
				Output:  []byte("Error response from daemon: something else went wrong"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isPoolOverlapError(tc.err)
			if result != tc.expected {
				t.Errorf("isPoolOverlapError(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestIsIPv6UnavailableError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "non-RunError returns false",
			err:      fmt.Errorf("some random error"),
			expected: false,
		},
		{
			name: "RunError with IPv6 unavailable message returns true",
			err: &exec.RunError{
				Command: []string{"nerdctl", "network", "create"},
				Output:  []byte("Error response from daemon: Cannot read IPv6 setup for bridge network"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with unrelated message returns false",
			err: &exec.RunError{
				Command: []string{"nerdctl", "network", "create"},
				Output:  []byte("Error: something else"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isIPv6UnavailableError(tc.err)
			if result != tc.expected {
				t.Errorf("isIPv6UnavailableError(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}
