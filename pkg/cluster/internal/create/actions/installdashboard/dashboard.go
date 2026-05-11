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

// Package installdashboard implements the action to install the Headlamp
// Kubernetes dashboard with RBAC, long-lived token, and access info printing.
package installdashboard

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/headlamp.yaml
var headlampManifest string

// Images contains the container images used by the Dashboard (Headlamp).
var Images = []string{
	"ghcr.io/headlamp-k8s/headlamp:v0.42.0",
}

type action struct{}

// NewAction returns a new action for installing the Dashboard
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing Dashboard")
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

	// Apply Headlamp manifest (SA, RBAC, Secret, Service, Deployment)
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(headlampManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply Headlamp manifest")
	}

	// Wait for Headlamp Deployment to be Available
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"wait",
		"--namespace=kube-system",
		"--for=condition=Available",
		"deployment/headlamp",
		"--timeout=120s",
	).Run(); err != nil {
		return errors.Wrap(err, "Headlamp deployment did not become available")
	}

	// Read the long-lived token from the Secret
	var tokenBuf bytes.Buffer
	if err := node.CommandContext(ctx.Context,
		"kubectl",
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"get", "secret", "kinder-dashboard-token",
		"--namespace=kube-system",
		"-o", "jsonpath={.data.token}",
	).SetStdout(&tokenBuf).Run(); err != nil {
		return errors.Wrap(err, "failed to read dashboard token from secret")
	}

	// Decode base64 in Go to avoid shell compatibility issues
	tokenBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(tokenBuf.String()))
	if err != nil {
		return errors.Wrap(err, "failed to decode dashboard token")
	}
	token := string(tokenBytes)

	// End spinner before printing multi-line output
	ctx.Status.End(true)

	// Print token and port-forward command for the user
	ctx.Logger.V(0).Info("")
	ctx.Logger.V(0).Info("Dashboard:")
	ctx.Logger.V(1).Infof("  Token: %s", token)
	ctx.Logger.V(0).Info("  Get token: kubectl -n kube-system get secret kinder-dashboard-token -o jsonpath='{.data.token}' | base64 -d")
	ctx.Logger.V(0).Info("  Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80")
	ctx.Logger.V(0).Info("  Then open: http://localhost:8080")
	ctx.Logger.V(0).Info("")

	return nil
}
