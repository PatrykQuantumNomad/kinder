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
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcertmanager"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installdashboard"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installenvoygw"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installlocalpath"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installlocalregistry"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetallb"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetricsserver"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installnvidiagpu"
	"sigs.k8s.io/kind/pkg/cluster/internal/loadbalancer"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/sets"
)

// RequiredNodeImages returns the set of _node_ images specified by the config
// This does not include the loadbalancer image, and is only used to improve
// the UX by explicit pulling the node images prior to running
func RequiredNodeImages(cfg *config.Cluster) sets.String {
	images := sets.NewString()
	for _, node := range cfg.Nodes {
		images.Insert(node.Image)
	}
	return images
}

// RequiredAddonImages returns the set of images required by enabled addons.
// This includes the LB image when the cluster has an implicit load balancer.
func RequiredAddonImages(cfg *config.Cluster) sets.String {
	images := sets.NewString()
	// LB image is needed for HA clusters (multiple control-plane nodes)
	if config.ClusterHasImplicitLoadBalancer(cfg) {
		images.Insert(loadbalancer.Image)
	}
	if cfg.Addons.LocalRegistry {
		images.Insert(installlocalregistry.Images...)
	}
	if cfg.Addons.MetalLB {
		images.Insert(installmetallb.Images...)
	}
	if cfg.Addons.MetricsServer {
		images.Insert(installmetricsserver.Images...)
	}
	if cfg.Addons.CertManager {
		images.Insert(installcertmanager.Images...)
	}
	if cfg.Addons.EnvoyGateway {
		images.Insert(installenvoygw.Images...)
	}
	if cfg.Addons.Dashboard {
		images.Insert(installdashboard.Images...)
	}
	if cfg.Addons.LocalPath {
		images.Insert(installlocalpath.Images...)
	}
	if cfg.Addons.NvidiaGPU {
		images.Insert(installnvidiagpu.Images...)
	}
	return images
}

// RequiredAllImages returns the union of node images and addon images.
func RequiredAllImages(cfg *config.Cluster) sets.String {
	all := RequiredNodeImages(cfg)
	for _, img := range RequiredAddonImages(cfg).List() {
		all.Insert(img)
	}
	return all
}
