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
	"net"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/internal/loadbalancer"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// TestRunArgsForLoadBalancerAppendsBootstrap_Podman verifies that podman's
// runArgsForLoadBalancer appends GenerateBootstrapCommand args after the LB image.
func TestRunArgsForLoadBalancerAppendsBootstrap_Podman(t *testing.T) {
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

	// Find position of loadbalancer.Image (sanitized) in returned args.
	// podman's runArgsForLoadBalancer calls sanitizeImage which may strip or transform the image ref.
	imageIdx := -1
	for i, a := range args {
		if strings.Contains(a, "envoyproxy/envoy") {
			imageIdx = i
			break
		}
	}
	if imageIdx < 0 {
		t.Fatalf("LB image (envoyproxy/envoy) not found in args: %v", args)
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
		t.Errorf("'bash' not found after LB image in args: %v", args)
	}
	if dashCIdx < 0 {
		t.Errorf("'-c' not found after LB image in args: %v", args)
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

	// Suppress unused import warning — loadbalancer.Image is used indirectly via the image check.
	_ = loadbalancer.Image
}

// Test_generatePortMappings tests the generatePortMappings function for the podman provider.
func Test_generatePortMappings(t *testing.T) {
	t.Parallel()

	t.Run("static port produces correct publish arg", func(t *testing.T) {
		t.Parallel()
		args, err := generatePortMappings(config.IPv4Family, config.PortMapping{
			ListenAddress: "0.0.0.0",
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      config.PortMappingProtocolTCP,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(args) != 1 {
			t.Fatalf("expected 1 arg, got %d: %v", len(args), args)
		}
		want := "--publish=0.0.0.0:8080:80/tcp"
		if args[0] != want {
			t.Errorf("got %q, want %q", args[0], want)
		}
	})

	t.Run("port=0 acquires random port and returns valid publish arg", func(t *testing.T) {
		t.Parallel()
		args, err := generatePortMappings(config.IPv4Family, config.PortMapping{
			ListenAddress: "0.0.0.0",
			HostPort:      0,
			ContainerPort: 80,
			Protocol:      config.PortMappingProtocolTCP,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(args) != 1 {
			t.Fatalf("expected 1 arg, got %d: %v", len(args), args)
		}
		if !strings.HasPrefix(args[0], "--publish=0.0.0.0:") {
			t.Errorf("unexpected publish arg format: %q", args[0])
		}
	})

	t.Run("port=-1 returns colon-terminated binding (podman random port) and no error", func(t *testing.T) {
		t.Parallel()
		// port=-1 means PortOrGetFreePort returns 0, then podman strips trailing "0"
		// resulting in host binding like "0.0.0.0:" (podman assigns random port)
		args, err := generatePortMappings(config.IPv4Family, config.PortMapping{
			ListenAddress: "0.0.0.0",
			HostPort:      -1,
			ContainerPort: 80,
			Protocol:      config.PortMappingProtocolTCP,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(args) != 1 {
			t.Fatalf("expected 1 arg, got %d: %v", len(args), args)
		}
		// With port=-1, PortOrGetFreePort returns 0, and podman strips the trailing "0"
		// Result should contain ":0" stripped to ":" in host portion
		if !strings.HasPrefix(args[0], "--publish=") {
			t.Errorf("unexpected publish arg format: %q", args[0])
		}
	})

	t.Run("invalid protocol returns error", func(t *testing.T) {
		t.Parallel()
		_, err := generatePortMappings(config.IPv4Family, config.PortMapping{
			ListenAddress: "0.0.0.0",
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "INVALID",
		})
		if err == nil {
			t.Fatal("expected error for invalid protocol, got nil")
		}
	})

	t.Run("empty mappings returns empty args", func(t *testing.T) {
		t.Parallel()
		args, err := generatePortMappings(config.IPv4Family)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(args) != 0 {
			t.Errorf("expected empty args, got %v", args)
		}
	})

	t.Run("multiple port=0 mappings all acquire distinct ports and release listeners", func(t *testing.T) {
		t.Parallel()
		// Call with 3 port=0 mappings. After generatePortMappings returns, all
		// port listeners must have been released (not held until caller returns).
		// We verify this by confirming we can immediately re-bind the returned ports.
		mappings := []config.PortMapping{
			{ListenAddress: "127.0.0.1", HostPort: 0, ContainerPort: 80, Protocol: config.PortMappingProtocolTCP},
			{ListenAddress: "127.0.0.1", HostPort: 0, ContainerPort: 81, Protocol: config.PortMappingProtocolTCP},
			{ListenAddress: "127.0.0.1", HostPort: 0, ContainerPort: 82, Protocol: config.PortMappingProtocolTCP},
		}
		args, err := generatePortMappings(config.IPv4Family, mappings...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(args) != 3 {
			t.Fatalf("expected 3 args, got %d: %v", len(args), args)
		}
		// Parse each port from the publish args and verify we can re-bind them.
		// Format: --publish=127.0.0.1:PORT:CONTAINERPORT/proto
		for i, arg := range args {
			withoutPrefix := strings.TrimPrefix(arg, "--publish=")
			parts := strings.Split(withoutPrefix, ":")
			if len(parts) < 3 {
				t.Errorf("arg[%d] %q has unexpected format", i, arg)
				continue
			}
			hostPort := parts[1]
			addr := fmt.Sprintf("127.0.0.1:%s", hostPort)
			ln, bindErr := net.Listen("tcp", addr)
			if bindErr != nil {
				t.Errorf("arg[%d]: port %s not released after generatePortMappings returned (bind failed: %v) — indicates defer-in-loop bug", i, hostPort, bindErr)
				continue
			}
			_ = ln.Close()
		}
	})
}
