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
	"net/netip"

	"sigs.k8s.io/kind/pkg/exec"
)

// subnetClashCheck detects when Docker network subnets overlap with host
// routing table entries (e.g., VPN subnets, corporate networks).
type subnetClashCheck struct {
	lookPath      func(string) (string, error)
	execCmd       func(name string, args ...string) exec.Cmd
	getRoutesFunc func() []string // injectable for testing
}

// newSubnetClashCheck creates a subnetClashCheck with real system deps.
func newSubnetClashCheck() Check {
	return &subnetClashCheck{}
}

func (c *subnetClashCheck) Name() string       { return "" }
func (c *subnetClashCheck) Category() string    { return "" }
func (c *subnetClashCheck) Platforms() []string { return nil }

func (c *subnetClashCheck) Run() []Result {
	return nil
}

// normalizeAbbreviatedCIDR expands macOS abbreviated route destinations
// to full CIDR notation.
func normalizeAbbreviatedCIDR(dest string) (netip.Prefix, bool) {
	return netip.Prefix{}, false
}
