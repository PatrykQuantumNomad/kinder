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

// Package constants contains well known constants for kind clusters
package constants

// DefaultClusterName is the default cluster Context name
const DefaultClusterName = "kind"

/* resume-strategy label constants (Plan 52-02) */

// ResumeStrategyLabel is the Docker/Podman label key written at container
// creation time to record the HA resume strategy for each CP container.
// Absence of the label on the bootstrap CP signals the legacy (pre-52-02) path
// which uses cert-regen at resume time (CONTEXT.md "legacy detection mechanism").
const ResumeStrategyLabel = "io.x-k8s.kinder.resume-strategy"

// StrategyIPPinned is the resume-strategy label value indicating that each
// CP container's IP has been pinned (IP survives stop/start).
// /kind/ipam-state.json records the assigned IPv4 address for resume use.
const StrategyIPPinned = "ip-pinned"

// StrategyCertRegen is the resume-strategy label value indicating that the
// cert-regen fallback will be used at resume time (IP pinning unavailable or
// probe returned a non-pinned verdict for this runtime).
const StrategyCertRegen = "cert-regen"

/* node role value constants */
const (
	// ControlPlaneNodeRoleValue identifies a node that hosts a Kubernetes
	// control-plane.
	//
	// NOTE: in single node clusters, control-plane nodes act as worker nodes
	ControlPlaneNodeRoleValue string = "control-plane"

	// WorkerNodeRoleValue identifies a node that hosts a Kubernetes worker
	WorkerNodeRoleValue string = "worker"

	// ExternalLoadBalancerNodeRoleValue identifies a node that hosts an
	// external load balancer for the API server in HA configurations.
	//
	// Please note that `kind` nodes hosting external load balancer are not
	// kubernetes nodes
	ExternalLoadBalancerNodeRoleValue string = "external-load-balancer"

	// ExternalEtcdNodeRoleValue identifies a node that hosts an external-etcd
	// instance.
	//
	// WARNING: this node type is not yet implemented!
	//
	// Please note that `kind` nodes hosting external etcd are not
	// kubernetes nodes
	ExternalEtcdNodeRoleValue string = "external-etcd"
)
