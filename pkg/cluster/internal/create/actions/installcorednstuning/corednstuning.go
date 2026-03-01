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

// Package installcorednstuning implements the action to tune CoreDNS cache
// settings for improved DNS performance.
package installcorednstuning

import (
	"bytes"
	"fmt"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
)

// configMapTemplate is the YAML envelope used to write back the patched Corefile.
const configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
%s`

type action struct{}

// NewAction returns a new action for tuning CoreDNS
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Tuning CoreDNS")
	defer ctx.Status.End(false)

	// Step 1: Get control plane node
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

	// Step 2: Read current Corefile
	var corefile bytes.Buffer
	if err := node.Command(
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"get", "configmap", "coredns",
		"--namespace=kube-system",
		"-o", "jsonpath={.data.Corefile}",
	).SetStdout(&corefile).Run(); err != nil {
		return errors.Wrap(err, "failed to read CoreDNS Corefile")
	}
	corefileStr := corefile.String()

	// Step 3: Guard checks — ensure expected strings are present before patching
	if !strings.Contains(corefileStr, "pods insecure") {
		return errors.New("CoreDNS Corefile does not contain 'pods insecure'; cannot patch safely")
	}
	if !strings.Contains(corefileStr, "cache 30") {
		return errors.New("CoreDNS Corefile does not contain 'cache 30'; cannot patch safely")
	}
	if !strings.Contains(corefileStr, "kubernetes cluster.local") {
		return errors.New("CoreDNS Corefile does not contain 'kubernetes cluster.local'; cannot patch safely")
	}

	// Step 4: Apply three string transforms
	// DNS-02: pods insecure -> pods verified
	corefileStr = strings.ReplaceAll(corefileStr, "pods insecure", "pods verified")
	// DNS-01: insert autopath @kubernetes before kubernetes cluster.local
	corefileStr = strings.ReplaceAll(corefileStr, "    kubernetes cluster.local", "    autopath @kubernetes\n    kubernetes cluster.local")
	// DNS-03: cache 30 -> cache 60
	corefileStr = strings.ReplaceAll(corefileStr, "cache 30", "cache 60")

	// Step 5: Write-back via ConfigMap YAML envelope
	configMapYAML := fmt.Sprintf(configMapTemplate, indentCorefile(corefileStr))
	if err := node.Command(
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(configMapYAML)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply patched CoreDNS ConfigMap")
	}

	// Step 6: Rollout restart and wait
	if err := node.Command(
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"rollout", "restart",
		"--namespace=kube-system",
		"deployment/coredns",
	).Run(); err != nil {
		return errors.Wrap(err, "failed to restart CoreDNS deployment")
	}
	if err := node.Command(
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"rollout", "status",
		"--namespace=kube-system",
		"deployment/coredns",
		"--timeout=60s",
	).Run(); err != nil {
		return errors.Wrap(err, "CoreDNS deployment did not become ready after restart")
	}

	ctx.Status.End(true)
	return nil
}

// indentCorefile prepends four spaces to each non-empty line of the Corefile
// so it can be embedded in the ConfigMap YAML literal block scalar.
func indentCorefile(corefile string) string {
	lines := strings.Split(corefile, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "    " + line
		}
	}
	return strings.Join(lines, "\n")
}
