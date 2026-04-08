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

package encoding

import (
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// V1Alpha4ToInternal converts to the internal API version
func V1Alpha4ToInternal(cluster *v1alpha4.Cluster) *config.Cluster {
	// Record which nodes have explicit images BEFORE defaults fill in empty ones.
	// SetDefaultsCluster sets Image = defaults.Image for any node with an empty Image field,
	// so after that call all nodes have a non-empty Image and we can't distinguish explicit
	// from defaulted. Capture the pre-defaults state here.
	explicitImage := make([]bool, len(cluster.Nodes))
	for i, n := range cluster.Nodes {
		explicitImage[i] = n.Image != ""
	}

	v1alpha4.SetDefaultsCluster(cluster)
	out := config.Convertv1alpha4(cluster)

	// Propagate the pre-defaults explicit image flags to the internal nodes.
	for i := range out.Nodes {
		if i < len(explicitImage) {
			out.Nodes[i].ExplicitImage = explicitImage[i]
		}
	}
	return out
}
