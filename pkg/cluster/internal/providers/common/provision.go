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

package common

import (
	"fmt"
	"net"
	"strings"

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// GenerateMountBindings converts the mount list to a list of args for docker/nerdctl.
// Format: '<HostPath>:<ContainerPath>[:options]', where 'options'
// is a comma-separated list of the following strings:
// 'ro', if the path is read only
// 'Z', if the volume requires SELinux relabeling
func GenerateMountBindings(mounts ...config.Mount) []string {
	args := make([]string, 0, len(mounts))
	for _, m := range mounts {
		bind := fmt.Sprintf("%s:%s", m.HostPath, m.ContainerPath)
		var attrs []string
		if m.Readonly {
			attrs = append(attrs, "ro")
		}
		// Only request relabeling if the pod provides an SELinux context. If the pod
		// does not provide an SELinux context relabeling will label the volume with
		// the container's randomly allocated MCS label. This would restrict access
		// to the volume to the container which mounts it first.
		if m.SelinuxRelabel {
			attrs = append(attrs, "Z")
		}
		switch m.Propagation {
		case config.MountPropagationNone:
			// noop, private is default
		case config.MountPropagationBidirectional:
			attrs = append(attrs, "rshared")
		case config.MountPropagationHostToContainer:
			attrs = append(attrs, "rslave")
		default: // Falls back to "private"
		}
		if len(attrs) > 0 {
			bind = fmt.Sprintf("%s:%s", bind, strings.Join(attrs, ","))
		}
		args = append(args, fmt.Sprintf("--volume=%s", bind))
	}
	return args
}

// GeneratePortMappings converts the portMappings list to a list of args for docker/nerdctl.
// Protocol is preserved as-is (uppercase TCP/UDP/SCTP as docker/nerdctl expect).
// Podman callers use their own generatePortMappings in podman/provision.go.
func GeneratePortMappings(clusterIPFamily config.ClusterIPFamily, portMappings ...config.PortMapping) ([]string, error) {
	args := make([]string, 0, len(portMappings))
	for _, pm := range portMappings {
		// do provider internal defaulting
		// in a future API revision we will handle this at the API level and remove this
		if pm.ListenAddress == "" {
			switch clusterIPFamily {
			case config.IPv4Family, config.DualStackFamily:
				pm.ListenAddress = "0.0.0.0" // this is the docker default anyhow
			case config.IPv6Family:
				pm.ListenAddress = "::"
			default:
				return nil, errors.Errorf("unknown cluster IP family: %v", clusterIPFamily)
			}
		}
		if string(pm.Protocol) == "" {
			pm.Protocol = config.PortMappingProtocolTCP // TCP is the default
		}

		// validate that the provider can handle this binding
		switch pm.Protocol {
		case config.PortMappingProtocolTCP:
		case config.PortMappingProtocolUDP:
		case config.PortMappingProtocolSCTP:
		default:
			return nil, errors.Errorf("unknown port mapping protocol: %v", pm.Protocol)
		}

		// get a random port if necessary (port = 0)
		hostPort, releaseHostPortFn, err := PortOrGetFreePort(pm.HostPort, pm.ListenAddress)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get random host port for port mapping")
		}

		// generate the actual mapping arg
		protocol := string(pm.Protocol)
		hostPortBinding := net.JoinHostPort(pm.ListenAddress, fmt.Sprintf("%d", hostPort))
		args = append(args, fmt.Sprintf("--publish=%s:%d/%s", hostPortBinding, pm.ContainerPort, protocol))

		// Release the port listener immediately — we have the port number,
		// the container runtime will bind it when the container starts.
		// Using defer here would hold all listeners open until function return,
		// which leaks file descriptors under high port-mapping counts.
		if releaseHostPortFn != nil {
			releaseHostPortFn()
		}
	}
	return args, nil
}

// CreateContainer creates a container using the given binaryName (e.g. "docker" or "nerdctl").
func CreateContainer(binaryName, name string, args []string) error {
	return exec.Command(binaryName, append([]string{"run", "--name", name}, args...)...).Run()
}
