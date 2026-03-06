/*
Copyright 2019 The Kubernetes Authors.

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

package doctor

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeAbbreviatedCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		// 1-octet macOS abbreviation
		{"127", "127.0.0.0/8", true},
		// 2-octet macOS abbreviation
		{"169.254", "169.254.0.0/16", true},
		// 3-octet macOS abbreviation
		{"192.168.86", "192.168.86.0/24", true},
		// Already CIDR, pass through
		{"10.0.0.0/8", "10.0.0.0/8", true},
		// Explicit CIDR host route
		{"192.168.86.1/32", "192.168.86.1/32", true},
		// 4-octet without CIDR = host route, skip
		{"192.168.86.1", "", false},
		// Non-numeric keywords
		{"default", "", false},
		{"link#23", "", false},
		// Empty string
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, ok := normalizeAbbreviatedCIDR(tt.input)
			if ok != tt.wantOK {
				t.Errorf("normalizeAbbreviatedCIDR(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
				return
			}
			if ok && got.String() != tt.want {
				t.Errorf("normalizeAbbreviatedCIDR(%q) = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestSubnetClashCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newSubnetClashCheck()
	if check.Name() != "network-subnet" {
		t.Errorf("Name() = %q, want %q", check.Name(), "network-subnet")
	}
	if check.Category() != "Network" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Network")
	}
	if check.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil", check.Platforms())
	}
}

func TestSubnetClashCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		lookPath        func(string) (string, error)
		execOutput      map[string]fakeExecResult
		wantStatus      string
		wantMsgContains string
	}{
		{
			name: "docker not found returns skip",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput:      map[string]fakeExecResult{},
			wantStatus:      "skip",
			wantMsgContains: "docker not found",
		},
		{
			name: "no Docker networks returns ok",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} kind":   {lines: "\n"},
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} bridge": {lines: "\n"},
			},
			wantStatus: "ok",
		},
		{
			name: "no host routes returns ok",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} kind":   {lines: "172.19.0.0/16 \n"},
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} bridge": {lines: "172.17.0.0/16 \n"},
			},
			wantStatus: "ok",
		},
		{
			name: "VPN overlap detected returns warn",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} kind":   {lines: "172.19.0.0/16 \n"},
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} bridge": {lines: "172.17.0.0/16 \n"},
			},
			wantStatus:      "warn",
			wantMsgContains: "overlaps",
		},
		{
			name: "self-referential Docker route skipped returns ok",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} kind":   {lines: "172.19.0.0/16 \n"},
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} bridge": {lines: "172.17.0.0/16 \n"},
			},
			wantStatus: "ok",
		},
		{
			name: "no overlap returns ok",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} kind":   {lines: "172.17.0.0/16 \n"},
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} bridge": {lines: "172.18.0.0/16 \n"},
			},
			wantStatus: "ok",
		},
		{
			name: "IPv6 subnets in Docker network are skipped",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} kind":   {lines: "172.19.0.0/16 fc00:f853:ccd:e793::/64 \n"},
				"docker network inspect --format {{range .IPAM.Config}}{{.Subnet}} {{end}} bridge": {lines: "172.17.0.0/16 \n"},
			},
			wantStatus: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &subnetClashCheck{
				lookPath:      tt.lookPath,
				execCmd:       newFakeExecCmd(tt.execOutput),
				getRoutesFunc: nil, // will be set per-test below
			}

			// Set up route function per test case
			switch tt.name {
			case "docker not found returns skip":
				// no routes needed
			case "no Docker networks returns ok":
				// no routes needed (no subnets to compare)
			case "no host routes returns ok":
				check.getRoutesFunc = fakeGetRoutes()
			case "VPN overlap detected returns warn":
				check.getRoutesFunc = fakeGetRoutes("172.16.0.0/12")
			case "self-referential Docker route skipped returns ok":
				check.getRoutesFunc = fakeGetRoutes("172.19.0.0/16")
			case "no overlap returns ok":
				check.getRoutesFunc = fakeGetRoutes("10.0.0.0/8")
			case "IPv6 subnets in Docker network are skipped":
				check.getRoutesFunc = fakeGetRoutes("10.0.0.0/8")
			}

			results := check.Run()
			if len(results) == 0 {
				t.Fatal("expected at least 1 result, got 0")
			}

			// For warn, check any result has warn status
			if tt.wantStatus == "warn" {
				found := false
				for _, r := range results {
					if r.Status == "warn" {
						found = true
						if tt.wantMsgContains != "" && !strings.Contains(r.Message, tt.wantMsgContains) {
							t.Errorf("warn Message = %q, want to contain %q", r.Message, tt.wantMsgContains)
						}
					}
				}
				if !found {
					t.Errorf("expected at least one warn result, got: %+v", results)
				}
				return
			}

			// For non-warn, check first result
			r := results[0]
			if r.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q (Message: %q)", r.Status, tt.wantStatus, r.Message)
			}
			if tt.wantMsgContains != "" && !strings.Contains(r.Message, tt.wantMsgContains) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContains)
			}
		})
	}
}

// fakeGetRoutes returns a getRoutesFunc that returns the given CIDR strings as prefixes.
func fakeGetRoutes(cidrs ...string) func() []string {
	return func() []string {
		return cidrs
	}
}
