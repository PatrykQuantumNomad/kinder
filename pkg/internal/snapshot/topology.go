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

package snapshot

// topology.go — CaptureTopology and CaptureAddonVersions.
//
// Design: ClassifyFn injection avoids importing lifecycle directly.
// Plan 04 will wire lifecycle.ClassifyNodes as the ClassifyFn argument when
// calling CaptureTopology, avoiding any circular import.
//
// CaptureAddonVersions queries each known addon's deployment image tag via
// kubectl --kubeconfig=/etc/kubernetes/admin.conf get deployment, matching the
// pattern from pkg/internal/doctor/localpath.go realGetProvisionerVersion.

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

// ClassifyFn is the function signature for node classification injected by the
// caller. lifecycle.ClassifyNodes satisfies this signature — Plan 04 wires it.
// Defining it here (not in lifecycle) avoids circular import.
type ClassifyFn func(allNodes []nodes.Node) (cp, workers []nodes.Node, lb nodes.Node, err error)

// AddonProbe describes how to discover a single addon's version from the cluster.
type AddonProbe struct {
	// CanonicalName is the map key written into metadata.json (e.g. "localPath").
	CanonicalName string
	// Namespace is the Kubernetes namespace of the addon's deployment.
	Namespace string
	// Deployment is the name of the addon's Deployment resource.
	Deployment string
}

// AddonRegistry is the canonical list of known kinder addons and their
// Kubernetes deployment coordinates. Each entry is probed by CaptureAddonVersions
// via `kubectl get deployment` inside the bootstrap CP node.
//
// Note: localRegistry is NOT a Kubernetes Deployment — it is a Docker container
// at the host level. It is intentionally omitted from this registry because its
// version is not discoverable via kubectl. The topology fingerprint covers it
// implicitly via nodeImage (which changes when registry mode is enabled).
var AddonRegistry = []AddonProbe{
	{"localPath", "local-path-storage", "local-path-provisioner"},
	{"metalLB", "metallb-system", "controller"},
	{"metricsServer", "kube-system", "metrics-server"},
	{"dashboard", "kubernetes-dashboard", "kubernetes-dashboard"},
	{"certManager", "cert-manager", "cert-manager"},
	{"envoyGateway", "envoy-gateway-system", "envoy-gateway"},
	{"coredns", "kube-system", "coredns"},
}

// CaptureTopology discovers the cluster's topology by:
//  1. Calling classify(allNodes) to get CP/worker/LB partition.
//  2. Reading /kind/version from cp[0] for the Kubernetes version string.
//  3. Calling `<providerBin> inspect --format {{.Config.Image}} <cp[0]>` for the node image.
//
// Returns (TopologyInfo, k8sVersion, nodeImage, error).
func CaptureTopology(ctx context.Context, allNodes []nodes.Node, classify ClassifyFn, providerBin string) (topo TopologyInfo, k8sVersion, nodeImage string, err error) {
	cp, workers, lb, err := classify(allNodes)
	if err != nil {
		return TopologyInfo{}, "", "", fmt.Errorf("CaptureTopology: classify nodes: %w", err)
	}
	if len(cp) == 0 {
		return TopologyInfo{}, "", "", fmt.Errorf("CaptureTopology: no control-plane nodes found")
	}

	bootstrap := cp[0]

	// Read k8s version from /kind/version inside the CP node.
	vLines, err := exec.OutputLines(bootstrap.CommandContext(ctx, "cat", "/kind/version"))
	if err != nil {
		return TopologyInfo{}, "", "", fmt.Errorf("CaptureTopology: read /kind/version: %w", err)
	}
	for _, l := range vLines {
		if v := strings.TrimSpace(l); v != "" {
			k8sVersion = v
			break
		}
	}

	// Get the node image via provider inspect.
	imgLines, err := exec.OutputLines(bootstrap.CommandContext(ctx,
		providerBin, "inspect", "--format", "{{.Config.Image}}", bootstrap.String(),
	))
	if err != nil {
		// Non-fatal: nodeImage may be empty if inspect fails (e.g. nerdctl differences).
		imgLines = nil
	}
	for _, l := range imgLines {
		if img := strings.TrimSpace(l); img != "" {
			nodeImage = img
			break
		}
	}

	topo = TopologyInfo{
		ControlPlaneCount: len(cp),
		WorkerCount:       len(workers),
		HasLoadBalancer:   lb != nil,
	}

	return topo, k8sVersion, nodeImage, nil
}

// CaptureAddonVersions queries each AddonRegistry entry via kubectl inside the
// bootstrap CP node and returns a map of canonical addon name → version tag.
// Only installed addons (where kubectl exits 0 and returns a non-empty image)
// appear in the result map.
//
// Version parsing: the image string from jsonpath may be "registry/repo:tag" —
// we return the full image string as the version value (tag extraction is
// caller-optional). An empty image string is treated as "not installed".
func CaptureAddonVersions(ctx context.Context, cp nodes.Node) (map[string]string, error) {
	result := make(map[string]string)

	for _, probe := range AddonRegistry {
		lines, err := exec.OutputLines(cp.CommandContext(ctx,
			"kubectl",
			"--kubeconfig=/etc/kubernetes/admin.conf",
			"get", "deployment", probe.Deployment,
			"-n", probe.Namespace,
			"-o", "jsonpath={.spec.template.spec.containers[0].image}",
		))
		if err != nil {
			// NotFound or any kubectl error → addon not installed; skip.
			continue
		}
		image := strings.TrimSpace(strings.Join(lines, ""))
		if image == "" {
			continue
		}
		// Parse the ":tag" suffix if present.
		version := image
		if idx := strings.LastIndex(image, ":"); idx >= 0 {
			version = image[idx+1:]
		}
		if version == "" {
			version = image
		}
		result[probe.CanonicalName] = version
	}

	return result, nil
}
