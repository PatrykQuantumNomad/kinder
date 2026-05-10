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

// Package installlocalpath implements the action to install local-path-provisioner
// for automatic PVC provisioning.
package installlocalpath

import (
	_ "embed"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/local-path-storage.yaml
var localPathManifest string

// Images contains the container images used by local-path-provisioner.
var Images = []string{
	"docker.io/rancher/local-path-provisioner:v0.0.36",
	"docker.io/library/busybox:1.37.0",
}

type action struct{}

// NewAction returns a new action for installing local-path-provisioner
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing Local Path Provisioner")
	defer ctx.Status.End(false)

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

	// Apply the embedded local-path-provisioner manifest via kubectl
	if err := node.CommandContext(ctx.Context, "kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(localPathManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply local-path-provisioner manifest")
	}

	// Wait for deployment readiness
	if err := node.CommandContext(ctx.Context, "kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"wait", "--namespace=local-path-storage",
		"--for=condition=Available", "deployment/local-path-provisioner",
		"--timeout=120s",
	).Run(); err != nil {
		return errors.Wrap(err, "local-path-provisioner deployment did not become available")
	}

	ctx.Status.End(true)
	return nil
}
