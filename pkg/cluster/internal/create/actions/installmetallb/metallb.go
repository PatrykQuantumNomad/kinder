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

// Package installmetallb implements the action to install MetalLB for
// LoadBalancer services.
package installmetallb

import (
	_ "embed"
	"fmt"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/metallb-native.yaml
var metalLBManifest string

// Images contains the container images used by MetalLB.
var Images = []string{
	"quay.io/metallb/controller:v0.15.3",
	"quay.io/metallb/speaker:v0.15.3",
}

const crTemplate = `---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kind-pool
  namespace: metallb-system
spec:
  addresses:
  - %s
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kind-l2advert
  namespace: metallb-system
spec:
  ipAddressPools:
  - kind-pool
`

type action struct{}

// NewAction returns a new action for installing MetalLB
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing MetalLB")
	defer ctx.Status.End(false)

	// Get control plane node
	allNodes, err := ctx.Nodes()
	if err != nil {
		return errors.Wrap(err, "failed to list cluster nodes")
	}
	controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
	if err != nil {
		return errors.Wrap(err, "failed to find control plane nodes")
	}
	if len(controlPlanes) == 0 {
		return errors.New("no control plane nodes found")
	}
	node := controlPlanes[0]

	// Check for rootless Podman and warn
	info, err := ctx.Provider.Info()
	if err != nil {
		return errors.Wrap(err, "failed to get provider info")
	}
	if info.Rootless {
		ctx.Logger.Warn("MetalLB L2 speaker cannot send ARP in rootless mode; LoadBalancer services may not receive EXTERNAL-IP")
	}

	// Detect provider name for subnet detection
	// Provider interface lacks String(); use fmt.Stringer type assertion
	providerName := "docker" // default fallback
	if s, ok := ctx.Provider.(fmt.Stringer); ok {
		providerName = s.String()
	}

	// Apply the embedded MetalLB manifest via kubectl
	if err := node.CommandContext(ctx.Context, "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-").
		SetStdin(strings.NewReader(metalLBManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply MetalLB manifest")
	}

	// Wait for controller webhook readiness
	if err := node.CommandContext(ctx.Context, "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"wait", "--namespace=metallb-system",
		"--for=condition=Available", "deployment/controller",
		"--timeout=120s").Run(); err != nil {
		return errors.Wrap(err, "MetalLB controller did not become available")
	}

	// Detect subnet from host network
	subnet, err := detectSubnet(providerName)
	if err != nil {
		return errors.Wrap(err, "failed to detect kind network subnet for MetalLB")
	}

	// Carve pool range
	poolRange, err := carvePoolFromSubnet(subnet)
	if err != nil {
		return errors.Wrap(err, "failed to compute MetalLB IP pool")
	}

	// Apply IPAddressPool + L2Advertisement CRs
	crYAML := fmt.Sprintf(crTemplate, poolRange)
	if err := node.CommandContext(ctx.Context, "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-").
		SetStdin(strings.NewReader(crYAML)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply MetalLB CRs")
	}

	ctx.Status.End(true)
	return nil
}
