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
	"fmt"
	"testing"

	"sigs.k8s.io/kind/pkg/exec"
)

func Test_isPoolOverlapError(t *testing.T) {
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
			name: "RunError with Pool overlaps prefix returns true",
			err: &exec.RunError{
				Command: []string{"docker", "network", "create"},
				Output:  []byte("Error response from daemon: Pool overlaps with other one on this address space"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with networks have overlapping returns true",
			err: &exec.RunError{
				Command: []string{"docker", "network", "create"},
				Output:  []byte("some prefix networks have overlapping IPv4"),
				Inner:   fmt.Errorf("exit status 1"),
			},
			expected: true,
		},
		{
			name: "RunError with unrelated message returns false",
			err: &exec.RunError{
				Command: []string{"docker", "network", "create"},
				Output:  []byte("Error response from daemon: something else entirely"),
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
				t.Errorf("isPoolOverlapError() = %v, want %v", result, tc.expected)
			}
		})
	}
}
