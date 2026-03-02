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
	"encoding/json"
	"fmt"
	"net"
	"os"

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

// dockerIPAMConfig holds a single IPAM configuration entry for Docker/Nerdctl networks.
type dockerIPAMConfig struct {
	Subnet  string `json:"Subnet"`
	Gateway string `json:"Gateway"`
}

// dockerIPAM holds the IPAM section of a Docker/Nerdctl network inspect entry.
type dockerIPAM struct {
	Config []dockerIPAMConfig `json:"Config"`
}

// dockerNetworkInspect represents one entry from `docker network inspect` or
// `nerdctl network inspect` output.
type dockerNetworkInspect struct {
	Name string     `json:"Name"`
	IPAM dockerIPAM `json:"IPAM"`
}

// podmanSubnet holds a single subnet entry in Podman network inspect output.
type podmanSubnet struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// podmanNetworkInspect represents one entry from `podman network inspect` output.
type podmanNetworkInspect struct {
	Name    string         `json:"name"`
	Subnets []podmanSubnet `json:"subnets"`
}

// parseSubnetFromJSON parses the JSON output of a network inspect command and
// returns the first IPv4 CIDR found. providerName controls which JSON schema is
// used: "podman" uses the Podman schema; anything else (docker, nerdctl) uses
// the Docker schema.
func parseSubnetFromJSON(output []byte, providerName string) (string, error) {
	if providerName == "podman" {
		return parsePodmanSubnet(output)
	}
	return parseDockerSubnet(output)
}

// parseDockerSubnet parses Docker/Nerdctl network inspect JSON.
func parseDockerSubnet(output []byte) (string, error) {
	var networks []dockerNetworkInspect
	if err := json.Unmarshal(output, &networks); err != nil {
		return "", errors.Wrap(err, "failed to parse docker network inspect JSON")
	}
	if len(networks) == 0 {
		return "", errors.New("no kind network found")
	}
	for _, cfg := range networks[0].IPAM.Config {
		if cfg.Subnet == "" {
			continue
		}
		ip, _, err := net.ParseCIDR(cfg.Subnet)
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			return cfg.Subnet, nil
		}
	}
	return "", errors.New("no IPv4 subnet found")
}

// parsePodmanSubnet parses Podman network inspect JSON.
func parsePodmanSubnet(output []byte) (string, error) {
	var networks []podmanNetworkInspect
	if err := json.Unmarshal(output, &networks); err != nil {
		return "", errors.Wrap(err, "failed to parse podman network inspect JSON")
	}
	if len(networks) == 0 {
		return "", errors.New("no kind network found")
	}
	for _, s := range networks[0].Subnets {
		if s.Subnet == "" {
			continue
		}
		ip, _, err := net.ParseCIDR(s.Subnet)
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			return s.Subnet, nil
		}
	}
	return "", errors.New("no IPv4 subnet found")
}

// carvePoolFromSubnet computes a MetalLB IP pool range from a CIDR. The pool
// covers the .200-.250 addresses within the last host-octet block of the subnet,
// for example:
//
//   - "172.18.0.0/16" -> "172.18.255.200-172.18.255.250"
//   - "10.89.0.0/24"  -> "10.89.0.200-10.89.0.250"
//   - "10.0.0.0/20"   -> "10.0.15.200-10.0.15.250"
func carvePoolFromSubnet(cidr string) (string, error) {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse CIDR")
	}
	// Reject IPv6.
	if ip.To4() == nil {
		return "", errors.New("only IPv4 subnets supported")
	}

	// Compute the broadcast address: network address OR (NOT mask).
	base := network.IP.To4()
	mask := network.Mask
	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = base[i] | ^mask[i]
	}

	// Validate that the subnet is large enough for pool carving.
	// We need at least a /24 (256 addresses) so that the .200-.250
	// range in the last octet is guaranteed to fall within the subnet.
	ones, bits := mask.Size()
	if bits-ones < 8 {
		return "", errors.Errorf("subnet %s is too small (/%d) for MetalLB pool carving; need at least /24", cidr, ones)
	}

	// Place pool at broadcast[-51] to broadcast[-1].
	start := make(net.IP, 4)
	end := make(net.IP, 4)
	copy(start, broadcast)
	copy(end, broadcast)
	start[3] = 200
	end[3] = 250

	return fmt.Sprintf("%s-%s", start.String(), end.String()), nil
}

// detectSubnet queries the container network provider for the "kind" network
// CIDR and returns it. The network name can be overridden via the
// KIND_EXPERIMENTAL_DOCKER_NETWORK environment variable.
func detectSubnet(providerName string) (string, error) {
	networkName := os.Getenv("KIND_EXPERIMENTAL_DOCKER_NETWORK")
	if networkName == "" {
		networkName = "kind"
	}

	output, err := exec.Output(exec.Command(providerName, "network", "inspect", networkName))
	if err != nil {
		return "", errors.Wrap(err, "failed to inspect container network")
	}

	subnet, err := parseSubnetFromJSON(output, providerName)
	if err != nil {
		return "", errors.Wrap(err, "failed to detect subnet from network inspect output")
	}
	return subnet, nil
}
