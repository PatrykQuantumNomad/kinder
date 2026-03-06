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
	"fmt"
	"net/netip"
	osexec "os/exec"
	"runtime"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
)

// subnetClashCheck detects when Docker network subnets overlap with host
// routing table entries (e.g., VPN subnets, corporate networks).
type subnetClashCheck struct {
	lookPath      func(string) (string, error)
	execCmd       func(name string, args ...string) exec.Cmd
	getRoutesFunc func() []string // injectable for testing; returns CIDR strings
}

// newSubnetClashCheck creates a subnetClashCheck with real system deps.
func newSubnetClashCheck() Check {
	c := &subnetClashCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
	c.getRoutesFunc = c.getHostRoutes
	return c
}

func (c *subnetClashCheck) Name() string       { return "network-subnet" }
func (c *subnetClashCheck) Category() string    { return "Network" }
func (c *subnetClashCheck) Platforms() []string { return nil }

func (c *subnetClashCheck) Run() []Result {
	// Step 1: Check if docker is available.
	if _, err := c.lookPath("docker"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(docker not found)",
		}}
	}

	// Step 2: Collect Docker network subnets from "kind" and "bridge" networks.
	var dockerSubnets []netip.Prefix
	for _, network := range []string{"kind", "bridge"} {
		subnets := c.getDockerNetworkSubnets(network)
		dockerSubnets = append(dockerSubnets, subnets...)
	}

	if len(dockerSubnets) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "no Docker network subnets found",
		}}
	}

	// Step 3: Get host routes.
	routeCIDRs := c.getRoutesFunc()
	var hostRoutes []netip.Prefix
	for _, cidr := range routeCIDRs {
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		if !p.Addr().Is4() {
			continue
		}
		hostRoutes = append(hostRoutes, p)
	}

	if len(hostRoutes) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  fmt.Sprintf("no host routes to compare (%d Docker subnets)", len(dockerSubnets)),
		}}
	}

	// Step 4: Check for overlaps, skipping self-referential routes.
	var results []Result
	for _, ds := range dockerSubnets {
		for _, hr := range hostRoutes {
			// Skip self-referential: Docker's own route in the host table.
			if ds == hr {
				continue
			}
			if ds.Overlaps(hr) {
				results = append(results, Result{
					Name:     c.Name(),
					Category: c.Category(),
					Status:   "warn",
					Message:  fmt.Sprintf("Docker subnet %s overlaps host route %s", ds, hr),
					Reason:   "Subnet overlap can cause connectivity issues between cluster nodes and host services (e.g., VPN, corporate network)",
					Fix:      fmt.Sprintf("Change Docker network subnet or check for VPN/route conflicts: docker network inspect kind"),
				})
			}
		}
	}

	if len(results) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  fmt.Sprintf("no subnet clashes (%d Docker subnets, %d host routes)", len(dockerSubnets), len(hostRoutes)),
		}}
	}

	return results
}

// getDockerNetworkSubnets retrieves IPv4 subnets for a Docker network.
func (c *subnetClashCheck) getDockerNetworkSubnets(networkName string) []netip.Prefix {
	lines, err := exec.OutputLines(c.execCmd(
		"docker", "network", "inspect",
		"--format", "{{range .IPAM.Config}}{{.Subnet}} {{end}}",
		networkName,
	))
	if err != nil || len(lines) == 0 {
		return nil
	}

	var prefixes []netip.Prefix
	for _, line := range lines {
		for _, field := range strings.Fields(line) {
			p, err := netip.ParsePrefix(field)
			if err != nil {
				continue
			}
			// Filter IPv4 only.
			if !p.Addr().Is4() {
				continue
			}
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}

// getHostRoutes retrieves IPv4 routes from the host routing table.
// On Linux, parses "ip route show"; on macOS, parses "netstat -rn".
func (c *subnetClashCheck) getHostRoutes() []string {
	if runtime.GOOS == "linux" {
		return c.getHostRoutesLinux()
	}
	return c.getHostRoutesDarwin()
}

// getHostRoutesLinux parses "ip route show" output.
// Each line starts with a destination CIDR like "172.17.0.0/16".
func (c *subnetClashCheck) getHostRoutesLinux() []string {
	lines, err := exec.OutputLines(c.execCmd("ip", "route", "show"))
	if err != nil {
		return nil
	}

	var routes []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		dest := fields[0]
		if dest == "default" {
			continue
		}
		p, err := netip.ParsePrefix(dest)
		if err != nil {
			continue
		}
		if !p.Addr().Is4() {
			continue
		}
		routes = append(routes, p.String())
	}
	return routes
}

// getHostRoutesDarwin parses "netstat -rn" output using normalizeAbbreviatedCIDR.
func (c *subnetClashCheck) getHostRoutesDarwin() []string {
	lines, err := exec.OutputLines(c.execCmd("netstat", "-rn"))
	if err != nil {
		return nil
	}

	var routes []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		dest := fields[0]
		p, ok := normalizeAbbreviatedCIDR(dest)
		if !ok {
			continue
		}
		if !p.Addr().Is4() {
			continue
		}
		routes = append(routes, p.String())
	}
	return routes
}

// normalizeAbbreviatedCIDR expands macOS abbreviated route destinations
// to full CIDR notation.
//
// macOS netstat -rn uses abbreviated forms:
//   - "127"        -> 127.0.0.0/8    (1-octet)
//   - "169.254"    -> 169.254.0.0/16 (2-octet)
//   - "192.168.86" -> 192.168.86.0/24 (3-octet)
//   - "10.0.0.0/8" -> 10.0.0.0/8     (already CIDR, pass through)
//
// Returns ok=false for: 4-octet without CIDR (host route), non-numeric
// strings ("default", "link#23"), and empty strings.
func normalizeAbbreviatedCIDR(dest string) (netip.Prefix, bool) {
	if dest == "" {
		return netip.Prefix{}, false
	}

	// If it already has a slash, try parsing as CIDR directly.
	if strings.Contains(dest, "/") {
		p, err := netip.ParsePrefix(dest)
		if err != nil {
			return netip.Prefix{}, false
		}
		return p, true
	}

	// Split by dots to count octets.
	parts := strings.Split(dest, ".")
	// Validate that the first part is numeric.
	if len(parts) == 0 {
		return netip.Prefix{}, false
	}
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return netip.Prefix{}, false
		}
	}

	switch len(parts) {
	case 1:
		// "127" -> "127.0.0.0/8"
		p, err := netip.ParsePrefix(dest + ".0.0.0/8")
		if err != nil {
			return netip.Prefix{}, false
		}
		return p, true
	case 2:
		// "169.254" -> "169.254.0.0/16"
		p, err := netip.ParsePrefix(dest + ".0.0/16")
		if err != nil {
			return netip.Prefix{}, false
		}
		return p, true
	case 3:
		// "192.168.86" -> "192.168.86.0/24"
		p, err := netip.ParsePrefix(dest + ".0/24")
		if err != nil {
			return netip.Prefix{}, false
		}
		return p, true
	case 4:
		// 4-octet without CIDR = host route (e.g., "192.168.86.1"), skip
		return netip.Prefix{}, false
	default:
		return netip.Prefix{}, false
	}
}
