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

package create

import (
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// TestFixupOptions_ExplicitImageOverride verifies that fixupOptions only overrides
// node images when ExplicitImage is false on the node.
func TestFixupOptions_ExplicitImageOverride(t *testing.T) {
	tests := []struct {
		name          string
		nodes         []config.Node
		nodeImage     string
		expectedImages []string
	}{
		{
			name: "mixed explicit and non-explicit nodes: only non-explicit overridden",
			nodes: []config.Node{
				{Role: config.ControlPlaneRole, Image: "kindest/node:v1.31.0", ExplicitImage: true},
				{Role: config.WorkerRole, Image: "kindest/node:v1.31.0", ExplicitImage: false},
				{Role: config.WorkerRole, Image: "kindest/node:v1.28.0", ExplicitImage: true},
			},
			nodeImage: "kindest/node:v1.32.0",
			expectedImages: []string{
				"kindest/node:v1.31.0", // cp: explicit, not overridden
				"kindest/node:v1.32.0", // worker: non-explicit, overridden
				"kindest/node:v1.28.0", // worker: explicit, not overridden
			},
		},
		{
			name: "all nodes have explicit images: no nodes overridden",
			nodes: []config.Node{
				{Role: config.ControlPlaneRole, Image: "kindest/node:v1.31.0", ExplicitImage: true},
				{Role: config.WorkerRole, Image: "kindest/node:v1.28.0", ExplicitImage: true},
			},
			nodeImage: "kindest/node:v1.32.0",
			expectedImages: []string{
				"kindest/node:v1.31.0",
				"kindest/node:v1.28.0",
			},
		},
		{
			name: "no explicit images: all nodes overridden (backward compat)",
			nodes: []config.Node{
				{Role: config.ControlPlaneRole, Image: "kindest/node:v1.31.0", ExplicitImage: false},
				{Role: config.WorkerRole, Image: "kindest/node:v1.31.0", ExplicitImage: false},
			},
			nodeImage: "kindest/node:v1.32.0",
			expectedImages: []string{
				"kindest/node:v1.32.0",
				"kindest/node:v1.32.0",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := &ClusterOptions{
				Config: &config.Cluster{
					Name:  "test",
					Nodes: tt.nodes,
				},
				NodeImage: tt.nodeImage,
			}

			if err := fixupOptions(opts); err != nil {
				t.Fatalf("fixupOptions returned unexpected error: %v", err)
			}

			if len(opts.Config.Nodes) != len(tt.expectedImages) {
				t.Fatalf("expected %d nodes, got %d", len(tt.expectedImages), len(opts.Config.Nodes))
			}

			for i, node := range opts.Config.Nodes {
				if node.Image != tt.expectedImages[i] {
					t.Errorf("node[%d]: expected image %q, got %q", i, tt.expectedImages[i], node.Image)
				}
			}
		})
	}
}
