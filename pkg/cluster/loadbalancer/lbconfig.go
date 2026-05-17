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

// Package loadbalancer provides public wrappers for the Envoy LB config
// helpers in pkg/cluster/internal/loadbalancer. This re-export exists
// because Go's internal package rule prevents pkg/internal/lifecycle from
// importing pkg/cluster/internal/loadbalancer directly (different subtrees).
// lifecycle.Resume() uses WriteDynamicConfig via this package.
//
// Phase 57.2 added ClusterIPFamily here as the single source of truth for
// resume-side IPv6 derivation, replacing the broken docker-network probe
// (`docker network inspect EnableIPv6`) that previously lived in
// pkg/internal/lifecycle and mis-classified IPv4 clusters as IPv6 on macOS
// Docker Desktop's dual-stack `kind` network.
package loadbalancer

import (
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/constants"
	internallb "sigs.k8s.io/kind/pkg/cluster/internal/loadbalancer"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// WriteDynamicConfig renders the LDS + CDS templates from the given control-
// plane node list and atomically swaps the rendered files into Envoy's
// config dir on `node`. Delegates to the canonical implementation in
// pkg/cluster/internal/loadbalancer so the atomic-swap mechanism has a
// single source of truth (CONTEXT.md D-lock 3, phase 57.1).
func WriteDynamicConfig(node nodes.Node, cps []nodes.Node, ipv6 bool) error {
	return internallb.WriteDynamicConfig(node, cps, ipv6)
}

// IPFamilyLabelValue converts a ClusterIPFamily into the label string written
// at container create time. Defaults to "ipv4" for empty/unrecognized inputs
// (matching the create-time default IPv4 behavior at the provider create.go).
//
// Exported so the provider create paths (docker/podman/nerdctl) can stamp the
// ip-family label without duplicating the mapping.
func IPFamilyLabelValue(f config.ClusterIPFamily) string {
	switch f {
	case config.IPv6Family:
		return constants.IPFamilyValueIPv6
	case config.DualStackFamily:
		return constants.IPFamilyValueDual
	default: // IPv4Family or unset
		return constants.IPFamilyValueIPv4
	}
}

// defaultClusterIPFamilyCmder is the host-side docker/podman cmder used by
// ClusterIPFamily. Production: exec.Command. Tests swap it via the same-
// package helper withClusterIPFamilyCmder (lbconfig_test.go) or via the
// exported SetClusterIPFamilyCmderForTest hook (cross-package use, e.g.
// pkg/internal/lifecycle tests).
var defaultClusterIPFamilyCmder = exec.Command

// SetClusterIPFamilyCmderForTest swaps the package-level cmder used by
// ClusterIPFamily and returns the previous value so callers can restore it
// in a t.Cleanup. Cross-package test hook only — not for production code.
// Tests using this hook MUST NOT call t.Parallel().
func SetClusterIPFamilyCmderForTest(c func(string, ...string) exec.Cmd) func(string, ...string) exec.Cmd {
	prev := defaultClusterIPFamilyCmder
	defaultClusterIPFamilyCmder = c
	return prev
}

// ClusterIPFamily reads the io.x-k8s.kinder.ip-family label from node and
// returns (true, nil) for ipv6 or dual, (false, nil) for ipv4. The label is
// stamped on every cluster container at create time (LB + CPs + workers) by
// the docker/podman/nerdctl provider create paths.
//
// If the label is absent or has an unrecognized value, returns a non-nil
// error whose message contains "delete and recreate the cluster". There
// is no fallback: pre-Phase-57.2 clusters are not supported; users delete
// and recreate.
//
// ClusterIPFamily NEVER calls `docker network inspect` — the cluster spec
// (encoded in the label) is the sole source of truth for IPv6 derivation.
// This is the architectural fix for the Phase 57.1 regression where the
// resume path mis-classified IPv4 clusters as IPv6 on macOS Docker Desktop's
// dual-stack kind network by reading docker `EnableIPv6` instead of the
// cluster's authoritative IPFamily; closed by Phase 57.2 on 2026-05-15.
// ClusterIPFamilyIsDual returns (true, nil) if the cluster is dual-stack
// (ipFamily: dual), (false, nil) for IPv4-only or IPv6-only clusters. Uses
// the same label-based probe as ClusterIPFamily. Introduced for Phase 57.3
// cert-regen: dual-stack etcd manifests use IPv4 loopback for --listen-client-urls
// (not IPv6 loopback) so the etcd health endpoint must be https://127.0.0.1:2379
// even though ClusterIPFamily returns ipv6=true for dual-stack clusters.
func ClusterIPFamilyIsDual(binaryName string, node nodes.Node) (bool, error) {
	var out strings.Builder
	cmd := defaultClusterIPFamilyCmder(binaryName,
		"inspect", "--format",
		`{{index .Config.Labels "`+constants.IPFamilyLabel+`"}}`,
		node.String(),
	)
	cmd.SetStdout(&out)
	if err := cmd.Run(); err != nil {
		return false, errors.Wrapf(err,
			"ClusterIPFamilyIsDual: %s inspect failed for %s", binaryName, node.String())
	}
	val := strings.TrimSpace(out.String())
	switch val {
	case constants.IPFamilyValueDual:
		return true, nil
	case constants.IPFamilyValueIPv4, constants.IPFamilyValueIPv6:
		return false, nil
	case "":
		return false, errors.Errorf(
			"ClusterIPFamilyIsDual: %s label absent on %s — delete and recreate the cluster",
			constants.IPFamilyLabel, node.String())
	default:
		return false, errors.Errorf(
			"ClusterIPFamilyIsDual: unrecognized %s label value %q on %s — delete and recreate the cluster",
			constants.IPFamilyLabel, val, node.String())
	}
}

func ClusterIPFamily(binaryName string, node nodes.Node) (bool, error) {
	var out strings.Builder
	cmd := defaultClusterIPFamilyCmder(binaryName,
		"inspect", "--format",
		`{{index .Config.Labels "`+constants.IPFamilyLabel+`"}}`,
		node.String(),
	)
	cmd.SetStdout(&out)
	if err := cmd.Run(); err != nil {
		return false, errors.Wrapf(err,
			"ClusterIPFamily: %s inspect failed for %s", binaryName, node.String())
	}
	val := strings.TrimSpace(out.String())
	switch val {
	case constants.IPFamilyValueIPv4:
		return false, nil
	case constants.IPFamilyValueIPv6, constants.IPFamilyValueDual:
		return true, nil
	case "":
		return false, errors.Errorf(
			"ClusterIPFamily: %s label absent on %s — delete and recreate the cluster",
			constants.IPFamilyLabel, node.String())
	default:
		return false, errors.Errorf(
			"ClusterIPFamily: unrecognized %s label value %q on %s — delete and recreate the cluster",
			constants.IPFamilyLabel, val, node.String())
	}
}
