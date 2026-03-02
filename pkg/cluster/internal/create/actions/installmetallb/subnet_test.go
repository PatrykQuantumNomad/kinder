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

package installmetallb

import (
	"testing"
)

func TestParseSubnetFromJSON(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		providerName string
		wantSubnet   string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "Docker JSON with IPv4 and IPv6 IPAM returns IPv4 CIDR",
			providerName: "docker",
			input:        []byte(`[{"Name":"kind","IPAM":{"Config":[{"Subnet":"172.18.0.0/16","Gateway":"172.18.0.1"},{"Subnet":"fc00:f853:ccd:e793::/64"}]}}]`),
			wantSubnet:   "172.18.0.0/16",
			wantErr:      false,
		},
		{
			name:         "Nerdctl JSON with IPv4 and IPv6 IPAM returns IPv4 CIDR",
			providerName: "nerdctl",
			input:        []byte(`[{"Name":"kind","IPAM":{"Config":[{"Subnet":"172.19.0.0/16","Gateway":"172.19.0.1"},{"Subnet":"fc00:f853:ccd:e793::/64"}]}}]`),
			wantSubnet:   "172.19.0.0/16",
			wantErr:      false,
		},
		{
			name:         "Docker JSON with only IPv6 IPAM returns error",
			providerName: "docker",
			input:        []byte(`[{"Name":"kind","IPAM":{"Config":[{"Subnet":"fc00:f853:ccd:e793::/64"}]}}]`),
			wantErr:      true,
			errContains:  "no IPv4 subnet found",
		},
		{
			name:         "Podman JSON with subnets array returns IPv4 CIDR",
			providerName: "podman",
			input:        []byte(`[{"name":"kind","subnets":[{"subnet":"10.89.0.0/24","gateway":"10.89.0.1"}]}]`),
			wantSubnet:   "10.89.0.0/24",
			wantErr:      false,
		},
		{
			name:         "Podman JSON with empty subnets returns error",
			providerName: "podman",
			input:        []byte(`[{"name":"kind","subnets":[]}]`),
			wantErr:      true,
			errContains:  "no IPv4 subnet found",
		},
		{
			name:         "Empty JSON array returns error",
			providerName: "docker",
			input:        []byte(`[]`),
			wantErr:      true,
			errContains:  "no kind network found",
		},
		{
			name:         "Malformed JSON returns error",
			providerName: "docker",
			input:        []byte(`{not valid json`),
			wantErr:      true,
		},
		{
			name:         "Podman malformed JSON returns error",
			providerName: "podman",
			input:        []byte(`not json at all`),
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSubnetFromJSON(tc.input, tc.providerName)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseSubnetFromJSON() expected error, got nil")
					return
				}
				if tc.errContains != "" {
					if !contains(err.Error(), tc.errContains) {
						t.Errorf("parseSubnetFromJSON() error = %q, want error containing %q", err.Error(), tc.errContains)
					}
				}
				return
			}
			if err != nil {
				t.Errorf("parseSubnetFromJSON() unexpected error: %v", err)
				return
			}
			if got != tc.wantSubnet {
				t.Errorf("parseSubnetFromJSON() = %q, want %q", got, tc.wantSubnet)
			}
		})
	}
}

func TestCarvePoolFromSubnet(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		wantPool    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "/16 network returns last .255.200-.255.250",
			cidr:     "172.18.0.0/16",
			wantPool: "172.18.255.200-172.18.255.250",
			wantErr:  false,
		},
		{
			name:     "/24 network returns .200-.250 in that subnet",
			cidr:     "10.89.0.0/24",
			wantPool: "10.89.0.200-10.89.0.250",
			wantErr:  false,
		},
		{
			name:     "/24 network minikube-style returns .200-.250",
			cidr:     "192.168.49.0/24",
			wantPool: "192.168.49.200-192.168.49.250",
			wantErr:  false,
		},
		{
			name:     "/16 network second range returns last .255.200-.255.250",
			cidr:     "172.19.0.0/16",
			wantPool: "172.19.255.200-172.19.255.250",
			wantErr:  false,
		},
		{
			name:     "/20 network returns .200-.250 in last octet block",
			cidr:     "10.0.0.0/20",
			wantPool: "10.0.15.200-10.0.15.250",
			wantErr:  false,
		},
		{
			name:    "invalid CIDR returns error",
			cidr:    "not-a-cidr",
			wantErr: true,
		},
		{
			name:        "IPv6 CIDR returns error",
			cidr:        "fc00:f853:ccd:e793::/64",
			wantErr:     true,
			errContains: "only IPv4 subnets supported",
		},
		{
			name:        "/25 subnet is too small for pool carving",
			cidr:        "10.0.0.0/25",
			wantErr:     true,
			errContains: "too small",
		},
		{
			name:        "/28 subnet is too small for pool carving",
			cidr:        "192.168.1.0/28",
			wantErr:     true,
			errContains: "too small",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := carvePoolFromSubnet(tc.cidr)
			if tc.wantErr {
				if err == nil {
					t.Errorf("carvePoolFromSubnet(%q) expected error, got nil", tc.cidr)
					return
				}
				if tc.errContains != "" {
					if !contains(err.Error(), tc.errContains) {
						t.Errorf("carvePoolFromSubnet(%q) error = %q, want error containing %q", tc.cidr, err.Error(), tc.errContains)
					}
				}
				return
			}
			if err != nil {
				t.Errorf("carvePoolFromSubnet(%q) unexpected error: %v", tc.cidr, err)
				return
			}
			if got != tc.wantPool {
				t.Errorf("carvePoolFromSubnet(%q) = %q, want %q", tc.cidr, got, tc.wantPool)
			}
		})
	}
}

// contains is a helper to check if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
