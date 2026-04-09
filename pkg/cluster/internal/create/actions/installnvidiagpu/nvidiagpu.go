/*
Copyright 2024 The Kubernetes Authors.

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

// Package installnvidiagpu implements the action to install the NVIDIA GPU device plugin and RuntimeClass.
package installnvidiagpu

import (
	_ "embed"
	"runtime"
	"strings"

	osexec "os/exec"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

//go:embed manifests/nvidia-runtimeclass.yaml
var runtimeClassManifest string

//go:embed manifests/nvidia-device-plugin.yaml
var devicePluginManifest string

// Images contains the container images used by the NVIDIA GPU device plugin.
var Images = []string{
	"nvcr.io/nvidia/k8s-device-plugin:v0.17.1",
}

// currentOS is the runtime OS, overridable in tests to bypass the platform guard.
var currentOS = runtime.GOOS

// checkPrerequisites validates host prerequisites. Package-level var so tests
// can inject a no-op to bypass LookPath checks on non-Linux dev/CI machines.
var checkPrerequisites = checkHostPrerequisites

type action struct{}

// NewAction returns a new action for installing the NVIDIA GPU device plugin and RuntimeClass.
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
	// Platform guard (GPU-04): skip silently on non-Linux platforms.
	// Use the package-level var (not runtime.GOOS directly) for test injection.
	if currentOS != "linux" {
		ctx.Logger.V(0).Infof(" * NVIDIA GPU addon: skipping on %s (Linux only)\n", currentOS)
		return nil
	}

	ctx.Status.Start("Installing NVIDIA GPU")
	defer ctx.Status.End(false)

	// Pre-flight check (GPU-06): validate host prerequisites before touching the cluster.
	if err := checkPrerequisites(); err != nil {
		return err
	}

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

	// Apply RuntimeClass first (GPU-02)
	if err := node.CommandContext(ctx.Context,
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(runtimeClassManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply NVIDIA RuntimeClass")
	}

	// Apply device plugin DaemonSet (GPU-01)
	if err := node.CommandContext(ctx.Context,
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(devicePluginManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply NVIDIA device plugin DaemonSet")
	}

	ctx.Status.End(true)
	return nil
}

// checkHostPrerequisites validates that the host has the required NVIDIA tooling
// and Docker runtime configuration for the GPU addon to function.
func checkHostPrerequisites() error {
	// Check 1: NVIDIA driver
	if _, err := osexec.LookPath("nvidia-smi"); err != nil {
		return errors.New("NVIDIA driver not found: install the NVIDIA driver for your GPU from https://www.nvidia.com/drivers")
	}
	// Check 2: nvidia-container-toolkit
	if _, err := osexec.LookPath("nvidia-ctk"); err != nil {
		return errors.New(
			"nvidia-container-toolkit not found: install it with:\n" +
				"  sudo apt-get install -y nvidia-container-toolkit\n" +
				"  sudo nvidia-ctk runtime configure --runtime=docker\n" +
				"  sudo systemctl restart docker",
		)
	}
	// Check 3: nvidia runtime configured in Docker
	if !nvidiaRuntimeInDocker() {
		return errors.New(
			"nvidia runtime not configured in Docker — run:\n" +
				"  sudo nvidia-ctk runtime configure --runtime=docker\n" +
				"  sudo systemctl restart docker",
		)
	}
	return nil
}

// nvidiaRuntimeInDocker checks whether the nvidia runtime is registered in Docker.
func nvidiaRuntimeInDocker() bool {
	out, err := exec.OutputLines(exec.Command("docker", "info", "--format",
		"{{range $k, $v := .Runtimes}}{{$k}} {{end}}"))
	if err != nil || len(out) == 0 {
		return false
	}
	return strings.Contains(out[0], "nvidia")
}
