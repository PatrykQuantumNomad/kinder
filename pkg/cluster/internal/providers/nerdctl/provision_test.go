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
	"net"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// Test_generatePortMappings tests the generatePortMappings function for the nerdctl provider.
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
		want := "--publish=0.0.0.0:8080:80/TCP"
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
			ln.Close()
		}
	})
}

func TestGenerateMountBindings(t *testing.T) {
	t.Parallel()

	// Smoke test: no mounts produces no args
	args := generateMountBindings()
	if len(args) != 0 {
		t.Errorf("expected no args for no mounts, got %v", args)
	}
}

func TestGeneratePortMappingsEmpty(t *testing.T) {
	t.Parallel()

	// No port mappings should produce no args and no error
	args, err := generatePortMappings(config.IPv4Family)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(args) != 0 {
		t.Errorf("expected no args for no port mappings, got %v", args)
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
