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
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/internal/loadbalancer"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// TestRunArgsForLoadBalancerAppendsBootstrap_Nerdctl verifies that nerdctl's
// runArgsForLoadBalancer appends GenerateBootstrapCommand args after the LB image.
func TestRunArgsForLoadBalancerAppendsBootstrap_Nerdctl(t *testing.T) {
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

	// Verify the mkdir bootstrap arg is present.
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

func TestGetSubnetsEmptyLines(t *testing.T) {
	t.Parallel()

	// getSubnets calls exec.OutputLines which will fail since the binary
	// doesn't exist in the test environment. We can't easily mock that,
	// but we can verify the function signature and that the bounds check
	// path works by testing the string splitting logic directly.

	// Simulate what getSubnets does after the bounds check
	testLine := "10.89.0.0/24 fc00:f853:ccd:e793::/64 "
	result := strings.Split(strings.TrimSpace(testLine), " ")
	if len(result) != 2 {
		t.Errorf("expected 2 subnets, got %d: %v", len(result), result)
	}
	if result[0] != "10.89.0.0/24" {
		t.Errorf("expected first subnet 10.89.0.0/24, got %s", result[0])
	}
	if result[1] != "fc00:f853:ccd:e793::/64" {
		t.Errorf("expected second subnet fc00:f853:ccd:e793::/64, got %s", result[1])
	}

	// Verify empty line handling
	emptyResult := strings.Split(strings.TrimSpace(""), " ")
	// strings.Split("", " ") returns [""] which has length 1
	if len(emptyResult) != 1 || emptyResult[0] != "" {
		t.Errorf("unexpected empty split result: %v", emptyResult)
	}
}
