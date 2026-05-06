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

// kindconfig.go — reconstruct a minimal kind v1alpha4 YAML from observed state.
//
// Design decision: we do NOT marshal through the real v1alpha4 API types
// (sigs.k8s.io/kind/pkg/apis/config) to keep the snapshot package free of
// cluster API imports and avoid potential import cycles. The output is a
// documentation artifact (YAML comment + hand-rolled string builder) — it is
// NOT intended for direct re-creation in this phase; it is written into the
// snapshot bundle as kind-config.yaml so operators can inspect what the cluster
// looked like when the snapshot was taken.

import (
	"fmt"
	"strings"
	"time"
)

// ReconstructKindConfig produces a minimal kind v1alpha4 Cluster YAML from the
// observed topology. Fields that cannot be recovered post-creation
// (ExtraMounts, ContainerdConfigPatches, FeatureGates, RuntimeConfig,
// KubeProxyMode) are listed in a prominent YAML comment.
//
// The HA load-balancer node is implicit in kind v1alpha4 (created automatically
// when controlPlaneCount >= 2); it does NOT get an explicit "role: load-balancer"
// entry because kind's v1alpha4 does not support that role value in the `nodes:`
// list. Its presence is documented via a comment when HasLoadBalancer is true.
func ReconstructKindConfig(topo TopologyInfo, k8sVersion, nodeImage string, addons map[string]string) ([]byte, error) {
	var b strings.Builder

	// Header comment
	fmt.Fprintf(&b, "# Reconstructed kind cluster config from snapshot\n")
	fmt.Fprintf(&b, "# Captured at: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "# k8sVersion: %s\n", k8sVersion)
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# NOTE: This config is reconstructed from live cluster state. Fields NOT\n")
	fmt.Fprintf(&b, "# observable post-creation are NOT included:\n")
	fmt.Fprintf(&b, "#   extraMounts, containerdConfigPatches, featureGates,\n")
	fmt.Fprintf(&b, "#   runtimeConfig, kubeProxyMode.\n")
	if topo.HasLoadBalancer {
		fmt.Fprintf(&b, "#\n")
		fmt.Fprintf(&b, "# NOTE: This cluster has an external load balancer (HA topology).\n")
		fmt.Fprintf(&b, "# kind creates the load balancer automatically when controlPlaneCount >= 2;\n")
		fmt.Fprintf(&b, "# it does not appear as a node entry in the v1alpha4 nodes list.\n")
	}
	fmt.Fprintf(&b, "#\n")

	// Kind cluster manifest
	fmt.Fprintf(&b, "kind: Cluster\n")
	fmt.Fprintf(&b, "apiVersion: kind.x-k8s.io/v1alpha4\n")
	fmt.Fprintf(&b, "nodes:\n")

	// Control-plane nodes
	for i := 0; i < topo.ControlPlaneCount; i++ {
		fmt.Fprintf(&b, "- role: control-plane\n")
		if nodeImage != "" {
			fmt.Fprintf(&b, "  image: %s\n", nodeImage)
		}
	}

	// Worker nodes
	for i := 0; i < topo.WorkerCount; i++ {
		fmt.Fprintf(&b, "- role: worker\n")
		if nodeImage != "" {
			fmt.Fprintf(&b, "  image: %s\n", nodeImage)
		}
	}

	return []byte(b.String()), nil
}
