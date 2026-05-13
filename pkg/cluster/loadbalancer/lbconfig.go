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
package loadbalancer

import (
	internallb "sigs.k8s.io/kind/pkg/cluster/internal/loadbalancer"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// WriteDynamicConfig renders the LDS + CDS templates from the given control-
// plane node list and atomically swaps the rendered files into Envoy's
// config dir on `node`. Delegates to the canonical implementation in
// pkg/cluster/internal/loadbalancer so the atomic-swap mechanism has a
// single source of truth (CONTEXT.md D-lock 3, phase 57.1).
func WriteDynamicConfig(node nodes.Node, cps []nodes.Node, ipv6 bool) error {
	return internallb.WriteDynamicConfig(node, cps, ipv6)
}
