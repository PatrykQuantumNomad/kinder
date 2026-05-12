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

// Package installenvoygw implements the action to install Envoy Gateway for
// Gateway API routing.
package installenvoygw

import (
	_ "embed"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/install.yaml
var envoyGWManifest string

// Images contains the container images used by Envoy Gateway.
var Images = []string{
	"docker.io/envoyproxy/ratelimit:05c08d03",
	"envoyproxy/gateway:v1.7.2",
}

const gatewayClassYAML = `apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
`

type action struct{}

// NewAction returns a new action for installing Envoy Gateway
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing Envoy Gateway")
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

	// Step 1: Apply Envoy Gateway install.yaml (Gateway API CRDs + controller)
	// --server-side is REQUIRED: httproutes CRD is 372 KB > 256 KB annotation limit for standard apply
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "--server-side", "-f", "-",
	).SetStdin(strings.NewReader(envoyGWManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply Envoy Gateway manifest")
	}

	// Step 2: Wait for certgen Job to complete (creates TLS Secrets for the controller)
	// Job name "eg-gateway-helm-certgen" verified for v1.3.1; re-check on upgrade
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"wait",
		"--namespace=envoy-gateway-system",
		"--for=condition=Complete",
		"job/eg-gateway-helm-certgen",
		"--timeout=120s",
	).Run(); err != nil {
		return errors.Wrap(err, "Envoy Gateway certgen job did not complete")
	}

	// Step 3: Wait for the Envoy Gateway controller Deployment to be Available
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"wait",
		"--namespace=envoy-gateway-system",
		"--for=condition=Available",
		"deployment/envoy-gateway",
		"--timeout=120s",
	).Run(); err != nil {
		return errors.Wrap(err, "Envoy Gateway controller did not become available")
	}

	// Step 4: Apply GatewayClass "eg" (not included in install.yaml)
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(gatewayClassYAML)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply Envoy Gateway GatewayClass")
	}

	// Step 5: Wait for GatewayClass to be accepted by the controller
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"wait",
		"--for=condition=Accepted",
		"gatewayclass/eg",
		"--timeout=60s",
	).Run(); err != nil {
		return errors.Wrap(err, "GatewayClass 'eg' was not accepted by Envoy Gateway controller")
	}

	ctx.Status.End(true)
	return nil
}
