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

// Package installcertmanager implements the action to install cert-manager with a self-signed ClusterIssuer.
package installcertmanager

import (
	_ "embed"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/cert-manager.yaml
var certManagerManifest string

// Images contains the container images used by cert-manager.
var Images = []string{
	"quay.io/jetstack/cert-manager-cainjector:v1.20.2",
	"quay.io/jetstack/cert-manager-controller:v1.20.2",
	"quay.io/jetstack/cert-manager-webhook:v1.20.2",
}

const selfSignedClusterIssuerYAML = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
`

type action struct{}

// NewAction returns a new action for installing cert-manager
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing Cert Manager 📜")
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

	// Step 1 (CERT-01): Apply cert-manager manifest with --server-side.
	// --server-side is REQUIRED: cert-manager CRDs exceed the 256KB last-applied-configuration
	// annotation limit imposed by standard kubectl apply.
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "--server-side", "-f", "-",
	).SetStdin(strings.NewReader(certManagerManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply cert-manager manifest")
	}

	// Step 2 (CERT-02): Wait for all three cert-manager deployments to reach Available.
	// The webhook MUST be Available before applying the ClusterIssuer — applying it while
	// the webhook is not ready causes "no endpoints available" errors.
	for _, deployment := range []string{
		"cert-manager",
		"cert-manager-cainjector",
		"cert-manager-webhook",
	} {
		if err := node.CommandContext(ctx.Context,
			"kubectl",
			"--kubeconfig=/etc/kubernetes/admin.conf",
			"wait", "--namespace=cert-manager",
			"--for=condition=Available",
			"deployment/"+deployment,
			"--timeout=300s",
		).Run(); err != nil {
			return errors.Wrapf(err, "cert-manager deployment %s did not become available", deployment)
		}
	}

	// Step 3 (CERT-03): Apply self-signed ClusterIssuer.
	// Standard client-side apply is correct here — ClusterIssuer is a small resource.
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(selfSignedClusterIssuerYAML)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply selfsigned ClusterIssuer")
	}

	ctx.Status.End(true)
	return nil
}
