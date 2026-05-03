/*
Copyright 2026 The Kubernetes Authors.

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

package lifecycle

import (
	osexec "os/exec"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/constants"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

// Cmder is the function signature used to construct an exec.Cmd. The package
// uses a package-level Cmder so unit tests can substitute a fake without
// shelling out to docker/podman/nerdctl.
type Cmder func(name string, args ...string) exec.Cmd

// defaultCmder is the production Cmder; tests swap it via t.Cleanup.
var defaultCmder Cmder = exec.Command

// ClusterLister is the minimum surface ResolveClusterName needs from a cluster
// provider. *cluster.Provider satisfies this interface.
type ClusterLister interface {
	List() ([]string, error)
}

// ContainerState returns the runtime State.Status (e.g. "running", "exited",
// "paused", "created", "dead") of the named container by invoking
// `<binaryName> inspect --format {{.State.Status}} <containerName>`.
//
// On any error from the runtime CLI ContainerState returns ("", err).
func ContainerState(binaryName, containerName string) (string, error) {
	if binaryName == "" {
		return "", errors.New("ContainerState: binaryName is empty (no container runtime detected)")
	}
	cmd := defaultCmder(binaryName, "inspect", "--format", "{{.State.Status}}", containerName)
	lines, err := exec.OutputLines(cmd)
	if err != nil {
		return "", errors.Wrapf(err, "failed to inspect container %q", containerName)
	}
	if len(lines) == 0 {
		return "", errors.Errorf("no output from %s inspect for container %q", binaryName, containerName)
	}
	return strings.TrimSpace(lines[0]), nil
}

// ClusterStatus aggregates per-node ContainerState results into a single
// status string suitable for the Status column on `kinder get clusters`.
//
// Mapping:
//   - "Running" → every node reports state "running"
//   - "Paused"  → every node reports a stopped state ("exited", "created", "dead")
//   - "Error"   → mixed states, empty node list, or any inspect error
func ClusterStatus(binaryName string, allNodes []nodes.Node) string {
	if len(allNodes) == 0 {
		return "Error"
	}
	allRunning := true
	allStopped := true
	for _, n := range allNodes {
		state, err := ContainerState(binaryName, n.String())
		if err != nil {
			return "Error"
		}
		switch state {
		case "running":
			allStopped = false
		case "exited", "created", "dead":
			allRunning = false
		default:
			// "paused" (docker freeze), "restarting", "removing", or unknown
			// values do not map cleanly onto Running/Paused — surface as Error
			// so the user investigates with `kinder status`.
			allRunning = false
			allStopped = false
		}
	}
	switch {
	case allRunning:
		return "Running"
	case allStopped:
		return "Paused"
	default:
		return "Error"
	}
}

// ResolveClusterName implements the no-argument auto-detect rule shared by
// `kinder pause`, `kinder resume`, and `kinder status`:
//
//   - len(args) == 1 → return args[0]
//   - len(args) == 0 and exactly one cluster exists → return that cluster name
//   - len(args) == 0 and zero clusters exist → error "no kind clusters found"
//   - len(args) == 0 and multiple clusters exist → error listing all names
//
// Cobra is responsible for rejecting len(args) > 1 via cobra.MaximumNArgs(1)
// before this function is reached; we still defend against it here.
func ResolveClusterName(args []string, lister ClusterLister) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if len(args) > 1 {
		return "", errors.Errorf("expected at most one cluster name argument, got %d", len(args))
	}
	names, err := lister.List()
	if err != nil {
		return "", errors.Wrap(err, "failed to list clusters")
	}
	switch len(names) {
	case 0:
		return "", errors.New("no kind clusters found")
	case 1:
		return names[0], nil
	default:
		return "", errors.Errorf("multiple clusters found; specify one: %s", strings.Join(names, ", "))
	}
}

// ClassifyNodes splits the node list by role into control-plane nodes
// (sorted via nodeutils.ControlPlaneNodes for deterministic ordering),
// worker nodes, and the optional external-load-balancer node.
//
// If any node returns an error from Role() the wrapped error is returned and
// the partial classification is discarded.
func ClassifyNodes(allNodes []nodes.Node) (cp, workers []nodes.Node, lb nodes.Node, err error) {
	cp, err = nodeutils.ControlPlaneNodes(allNodes)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to classify control-plane nodes")
	}
	workers, err = nodeutils.SelectNodesByRole(allNodes, constants.WorkerNodeRoleValue)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to classify worker nodes")
	}
	lb, err = nodeutils.ExternalLoadBalancerNode(allNodes)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to classify external load balancer")
	}
	return cp, workers, lb, nil
}

// ProviderBinaryName returns the first available container runtime binary
// found in $PATH ("docker", "podman", "nerdctl"), or "" if none are present.
//
// This mirrors the auto-detect logic in pkg/internal/doctor/clusterskew.go so
// callers that don't already have a *cluster.Provider can still issue
// container CLI commands (inspect, exec, stop, start) directly.
func ProviderBinaryName() string {
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := osexec.LookPath(rt); err == nil {
			return rt
		}
	}
	return ""
}
