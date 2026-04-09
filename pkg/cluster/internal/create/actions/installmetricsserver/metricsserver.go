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

// Package installmetricsserver implements the action to install Metrics Server
// for kubectl top and HPA metrics.
package installmetricsserver

import (
	_ "embed"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/components.yaml
var metricsServerManifest string

// Images contains the container images used by Metrics Server.
var Images = []string{
	"registry.k8s.io/metrics-server/metrics-server:v0.8.1",
}

type action struct{}

// NewAction returns a new action for installing Metrics Server
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing Metrics Server")
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

	// Apply the embedded Metrics Server manifest via kubectl
	if err := node.CommandContext(ctx.Context, "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-").
		SetStdin(strings.NewReader(metricsServerManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply Metrics Server manifest")
	}

	// Wait for deployment readiness
	if err := node.CommandContext(ctx.Context, "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"wait", "--namespace=kube-system",
		"--for=condition=Available", "deployment/metrics-server",
		"--timeout=120s").Run(); err != nil {
		return errors.Wrap(err, "Metrics Server deployment did not become available")
	}

	ctx.Status.End(true)
	return nil
}
