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
			name: "RunError with already used on host message returns true",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("subnet is already used on the host or by another config"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with used by network interface message returns true",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("subnet is being used by a network interface"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with used by cni config message returns true",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("subnet is already being used by a cni configuration"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with unrelated message returns false",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("some other error message"),
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

func TestIsUnknownIPv6FlagError(t *testing.T) {
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
			name: "RunError with unknown flag message returns true",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("Error: unknown flag: --ipv6"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with unrelated message returns false",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("Error: some other issue"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isUnknownIPv6FlagError(tc.err)
			if result != tc.expected {
				t.Errorf("isUnknownIPv6FlagError(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestIsIPv6DisabledError(t *testing.T) {
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
			name: "RunError with ipv6 disabled message returns true",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("Error: is ipv6 enabled in the kernel"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with unrelated message returns false",
			err: &exec.RunError{
				Command: []string{"podman", "network", "create"},
				Output:  []byte("Error: some other issue"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isIPv6DisabledError(tc.err)
			if result != tc.expected {
				t.Errorf("isIPv6DisabledError(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestGenerateULASubnetFromName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		attempt int32
		subnet  string
	}{
		{
			name:   "kind",
			subnet: "fc00:f853:ccd:e793::/64",
		},
		{
			name:    "foo",
			attempt: 1,
			subnet:  "fc00:8edf:7f02:ec8f::/64",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%s,%d", tc.name, tc.attempt), func(t *testing.T) {
			t.Parallel()
			subnet := generateULASubnetFromName(tc.name, tc.attempt)
			if subnet != tc.subnet {
				t.Errorf("generateULASubnetFromName(%q, %d) = %v, want %v", tc.name, tc.attempt, subnet, tc.subnet)
			}
		})
	}
}
