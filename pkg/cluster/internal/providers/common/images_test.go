/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or impliep.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"reflect"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcertmanager"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installdashboard"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installenvoygw"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installlocalregistry"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetallb"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetricsserver"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installnvidiagpu"
	"sigs.k8s.io/kind/pkg/cluster/internal/loadbalancer"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/sets"
)

func TestRequiredNodeImages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cluster *config.Cluster
		want    sets.String
	}{
		{
			name: "Cluster with different images",
			cluster: func() *config.Cluster {
				c := config.Cluster{}
				n, n2 := config.Node{}, config.Node{}
				n.Image = "node1"
				n2.Image = "node2"
				c.Nodes = []config.Node{n, n2}
				return &c
			}(),
			want: sets.NewString("node1", "node2"),
		},
		{
			name: "Cluster with nodes with same image",
			cluster: func() *config.Cluster {
				c := config.Cluster{}
				n, n2 := config.Node{}, config.Node{}
				n.Image = "node1"
				n2.Image = "node1"
				c.Nodes = []config.Node{n, n2}
				return &c
			}(),
			want: sets.NewString("node1"),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := RequiredNodeImages(tt.cluster); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RequiredNodeImages() = %v, want %v", got, tt.want)
			}
		})
	}
}

// singleCPCluster returns a cluster with one control-plane node (no implicit LB).
func singleCPCluster(addons config.Addons) *config.Cluster {
	return &config.Cluster{
		Nodes:  []config.Node{{Role: config.ControlPlaneRole, Image: "kindest/node:v1.30.0"}},
		Addons: addons,
	}
}

// multiCPCluster returns a cluster with 3 control-plane nodes (has implicit LB).
func multiCPCluster(addons config.Addons) *config.Cluster {
	return &config.Cluster{
		Nodes: []config.Node{
			{Role: config.ControlPlaneRole, Image: "kindest/node:v1.30.0"},
			{Role: config.ControlPlaneRole, Image: "kindest/node:v1.30.0"},
			{Role: config.ControlPlaneRole, Image: "kindest/node:v1.30.0"},
		},
		Addons: addons,
	}
}

func TestRequiredAddonImages_AllDisabled(t *testing.T) {
	t.Parallel()
	cfg := singleCPCluster(config.Addons{})
	got := RequiredAddonImages(cfg)
	if got.Len() != 0 {
		t.Errorf("expected empty set, got %v", got.List())
	}
}

func TestRequiredAddonImages_LBOnlyWithMultiCP(t *testing.T) {
	t.Parallel()
	cfg := multiCPCluster(config.Addons{})
	got := RequiredAddonImages(cfg)
	want := sets.NewString(loadbalancer.Image)
	if !got.Equal(want) {
		t.Errorf("RequiredAddonImages() = %v, want %v", got.List(), want.List())
	}
}

func TestRequiredAddonImages_MetalLBOnly(t *testing.T) {
	t.Parallel()
	cfg := singleCPCluster(config.Addons{MetalLB: true})
	got := RequiredAddonImages(cfg)
	want := sets.NewString(installmetallb.Images...)
	if !got.Equal(want) {
		t.Errorf("RequiredAddonImages() = %v, want %v", got.List(), want.List())
	}
}

func TestRequiredAddonImages_AllEnabled(t *testing.T) {
	t.Parallel()
	cfg := multiCPCluster(config.Addons{
		LocalRegistry: true,
		MetalLB:       true,
		MetricsServer: true,
		CertManager:   true,
		EnvoyGateway:  true,
		Dashboard:     true,
		NvidiaGPU:     true,
	})
	got := RequiredAddonImages(cfg)

	// Build expected set: LB + all addon images
	want := sets.NewString(loadbalancer.Image)
	want.Insert(installlocalregistry.Images...)
	want.Insert(installmetallb.Images...)
	want.Insert(installmetricsserver.Images...)
	want.Insert(installcertmanager.Images...)
	want.Insert(installenvoygw.Images...)
	want.Insert(installdashboard.Images...)
	want.Insert(installnvidiagpu.Images...)

	if !got.Equal(want) {
		t.Errorf("RequiredAddonImages() = %v, want %v", got.List(), want.List())
	}
}

func TestRequiredAllImages(t *testing.T) {
	t.Parallel()
	cfg := singleCPCluster(config.Addons{MetalLB: true})
	cfg.Nodes[0].Image = "kindest/node:v1.30.0"

	got := RequiredAllImages(cfg)

	want := sets.NewString("kindest/node:v1.30.0")
	want.Insert(installmetallb.Images...)

	if !got.Equal(want) {
		t.Errorf("RequiredAllImages() = %v, want %v", got.List(), want.List())
	}
}
