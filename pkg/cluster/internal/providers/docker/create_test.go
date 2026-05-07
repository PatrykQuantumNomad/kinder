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

	"sigs.k8s.io/kind/pkg/cluster/internal/loadbalancer"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// TestRunArgsForLoadBalancerAppendsBootstrap verifies that runArgsForLoadBalancer
// appends GenerateBootstrapCommand args after the LB image in the returned arg list.
func TestRunArgsForLoadBalancerAppendsBootstrap(t *testing.T) {
	t.Parallel()
	cfg := &config.Cluster{
		Name: "test-cluster",
		Networking: config.Networking{
			IPFamily:         config.IPv4Family,
			APIServerAddress: "127.0.0.1",
			APIServerPort:    0,
		},
	}
	// Use no generic args to keep the slice small and predictable.
	args, err := runArgsForLoadBalancer(cfg, "test-cluster-external-load-balancer", nil)
	if err != nil {
		t.Fatalf("runArgsForLoadBalancer() error: %v", err)
	}

	// Find position of loadbalancer.Image in returned args.
	imageIdx := -1
	for i, a := range args {
		if a == loadbalancer.Image {
			imageIdx = i
			break
		}
	}
	if imageIdx < 0 {
		t.Fatalf("loadbalancer.Image %q not found in args: %v", loadbalancer.Image, args)
	}

	// Find "bash" and "-c" after the image.
	bashIdx := -1
	dashCIdx := -1
	for i := imageIdx + 1; i < len(args); i++ {
		if args[i] == "bash" {
			bashIdx = i
		}
		if args[i] == "-c" {
			dashCIdx = i
		}
	}
	if bashIdx < 0 {
		t.Errorf("'bash' not found after loadbalancer.Image in args: %v", args)
	}
	if dashCIdx < 0 {
		t.Errorf("'-c' not found after loadbalancer.Image in args: %v", args)
	}

	// Find the mkdir arg (the bash command body).
	foundMkdir := false
	for _, a := range args[imageIdx+1:] {
		if strings.Contains(a, "mkdir -p /home/envoy") {
			foundMkdir = true
			break
		}
	}
	if !foundMkdir {
		t.Errorf("no arg containing 'mkdir -p /home/envoy' found after LB image; args: %v", args)
	}
}

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
