/*
Copyright 2018 The Kubernetes Authors.

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
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// TestExtractMountPaths covers the extractMountPaths helper.
func TestExtractMountPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		nodes []config.Node
		want  []string
	}{
		{
			name:  "no nodes returns nil",
			nodes: nil,
			want:  nil,
		},
		{
			name: "no extraMounts returns nil",
			nodes: []config.Node{
				{Role: config.ControlPlaneRole},
			},
			want: nil,
		},
		{
			name: "single node single mount",
			nodes: []config.Node{
				{
					Role: config.ControlPlaneRole,
					ExtraMounts: []config.Mount{
						{HostPath: "/data/project", ContainerPath: "/mnt/project"},
					},
				},
			},
			want: []string{"/data/project"},
		},
		{
			name: "multiple nodes with overlapping host paths deduplicated",
			nodes: []config.Node{
				{
					Role: config.ControlPlaneRole,
					ExtraMounts: []config.Mount{
						{HostPath: "/data/shared", ContainerPath: "/mnt/shared"},
						{HostPath: "/data/control", ContainerPath: "/mnt/control"},
					},
				},
				{
					Role: config.WorkerRole,
					ExtraMounts: []config.Mount{
						{HostPath: "/data/shared", ContainerPath: "/mnt/shared"},
						{HostPath: "/data/worker", ContainerPath: "/mnt/worker"},
					},
				},
			},
			// /data/shared appears in both nodes but should only appear once.
			want: []string{"/data/shared", "/data/control", "/data/worker"},
		},
		{
			name: "multiple mounts across nodes all unique",
			nodes: []config.Node{
				{
					Role: config.ControlPlaneRole,
					ExtraMounts: []config.Mount{
						{HostPath: "/alpha", ContainerPath: "/mnt/alpha"},
					},
				},
				{
					Role: config.WorkerRole,
					ExtraMounts: []config.Mount{
						{HostPath: "/beta", ContainerPath: "/mnt/beta"},
					},
				},
			},
			want: []string{"/alpha", "/beta"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cluster := &config.Cluster{Nodes: tt.nodes}
			got := extractMountPaths(cluster)
			if len(got) != len(tt.want) {
				t.Fatalf("extractMountPaths() = %v, want %v", got, tt.want)
			}
			for i, p := range got {
				if p != tt.want[i] {
					t.Errorf("extractMountPaths()[%d] = %q, want %q", i, p, tt.want[i])
				}
			}
		})
	}
}
