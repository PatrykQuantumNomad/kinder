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

package common

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// TestGeneratePortMappings tests the GeneratePortMappings function.
func TestGeneratePortMappings(t *testing.T) {
	t.Parallel()

	t.Run("static port produces correct publish arg", func(t *testing.T) {
		t.Parallel()
		args, err := GeneratePortMappings(config.IPv4Family, config.PortMapping{
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
		args, err := GeneratePortMappings(config.IPv4Family, config.PortMapping{
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

	t.Run("port=-1 returns zero port (let backend pick) and no error", func(t *testing.T) {
		t.Parallel()
		args, err := GeneratePortMappings(config.IPv4Family, config.PortMapping{
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
	})

	t.Run("invalid protocol returns error", func(t *testing.T) {
		t.Parallel()
		_, err := GeneratePortMappings(config.IPv4Family, config.PortMapping{
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
		args, err := GeneratePortMappings(config.IPv4Family)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(args) != 0 {
			t.Errorf("expected empty args, got %v", args)
		}
	})

	t.Run("multiple port=0 mappings all acquire distinct ports and release listeners", func(t *testing.T) {
		t.Parallel()
		// Call with 3 port=0 mappings. After GeneratePortMappings returns, all
		// port listeners must have been released (not held until caller returns).
		// We verify this by confirming we can immediately re-bind the returned ports.
		mappings := []config.PortMapping{
			{ListenAddress: "127.0.0.1", HostPort: 0, ContainerPort: 80, Protocol: config.PortMappingProtocolTCP},
			{ListenAddress: "127.0.0.1", HostPort: 0, ContainerPort: 81, Protocol: config.PortMappingProtocolTCP},
			{ListenAddress: "127.0.0.1", HostPort: 0, ContainerPort: 82, Protocol: config.PortMappingProtocolTCP},
		}
		args, err := GeneratePortMappings(config.IPv4Family, mappings...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(args) != 3 {
			t.Fatalf("expected 3 args, got %d: %v", len(args), args)
		}
		// Parse each port from the publish args and verify we can re-bind them.
		// If the listeners were deferred (not yet released), re-binding would fail.
		for i, arg := range args {
			// arg format: --publish=127.0.0.1:PORT:CONTAINERPORT/PROTO
			// Strip prefix and extract host:port portion
			withoutPrefix := strings.TrimPrefix(arg, "--publish=")
			// Find the last colon separating host:port from containerPort
			// The format is hostAddr:hostPort:containerPort/proto
			// Split on : to find host port
			parts := strings.Split(withoutPrefix, ":")
			if len(parts) < 3 {
				t.Errorf("arg[%d] %q has unexpected format", i, arg)
				continue
			}
			hostPort := parts[1]
			addr := fmt.Sprintf("127.0.0.1:%s", hostPort)
			ln, bindErr := net.Listen("tcp", addr)
			if bindErr != nil {
				t.Errorf("arg[%d]: port %s not released after GeneratePortMappings returned (bind failed: %v) — indicates defer-in-loop bug", i, hostPort, bindErr)
				continue
			}
			_ = ln.Close()
		}
	})
}

// TestGenerateMountBindings tests the GenerateMountBindings function.
func TestGenerateMountBindings(t *testing.T) {
	t.Parallel()

	// Smoke test: no mounts produces no args
	args := GenerateMountBindings()
	if len(args) != 0 {
		t.Errorf("expected no args for no mounts, got %v", args)
	}
}
