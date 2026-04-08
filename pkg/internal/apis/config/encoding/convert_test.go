/*
Copyright 2024 The Kubernetes Authors.

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
	"testing"

	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

// TestV1Alpha4ToInternal_ExplicitImage verifies that ExplicitImage is correctly
// set based on whether the user provided an image in the v1alpha4 config
// BEFORE defaults are applied.
func TestV1Alpha4ToInternal_ExplicitImage(t *testing.T) {
	tests := []struct {
		name          string
		nodes         []v1alpha4.Node
		wantExplicit  []bool
	}{
		{
			name: "node with explicit image sets ExplicitImage=true",
			nodes: []v1alpha4.Node{
				{Role: v1alpha4.ControlPlaneRole, Image: "kindest/node:v1.31.0"},
			},
			wantExplicit: []bool{true},
		},
		{
			name: "node with empty image sets ExplicitImage=false (defaults applied)",
			nodes: []v1alpha4.Node{
				{Role: v1alpha4.ControlPlaneRole, Image: ""},
			},
			wantExplicit: []bool{false},
		},
		{
			name: "mixed nodes: explicit and non-explicit",
			nodes: []v1alpha4.Node{
				{Role: v1alpha4.ControlPlaneRole, Image: "kindest/node:v1.31.0"},
				{Role: v1alpha4.WorkerRole, Image: ""},
				{Role: v1alpha4.WorkerRole, Image: "kindest/node:v1.28.0"},
			},
			wantExplicit: []bool{true, false, true},
		},
		{
			name:         "all nodes with empty images: all ExplicitImage=false",
			nodes:        []v1alpha4.Node{
				{Role: v1alpha4.ControlPlaneRole, Image: ""},
				{Role: v1alpha4.WorkerRole, Image: ""},
			},
			wantExplicit: []bool{false, false},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cluster := &v1alpha4.Cluster{
				Nodes: tt.nodes,
			}

			out := V1Alpha4ToInternal(cluster)

			if len(out.Nodes) != len(tt.wantExplicit) {
				t.Fatalf("expected %d nodes, got %d", len(tt.wantExplicit), len(out.Nodes))
			}

			for i, node := range out.Nodes {
				if node.ExplicitImage != tt.wantExplicit[i] {
					t.Errorf("node[%d]: expected ExplicitImage=%v, got ExplicitImage=%v (image=%q)",
						i, tt.wantExplicit[i], node.ExplicitImage, node.Image)
				}
				// Nodes without explicit image should still have Image set (from defaults)
				if !tt.wantExplicit[i] && node.Image == "" {
					t.Errorf("node[%d]: non-explicit node should have Image set via defaults, got empty string", i)
				}
			}
		})
	}
}
