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

// Package installlocalregistry implements the action to start a local
// container registry and wire it into all cluster nodes for image pull
// via localhost:5001.
package installlocalregistry

import (
	_ "embed"
	"fmt"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

//go:embed manifests/local-registry-hosting.yaml
var localRegistryHostingManifest string

const (
	registryName    = "kind-registry"
	registryPort    = "5001" // host-published port
	registryIntPort = "5000" // container-internal port
	registryImage   = "registry:2"
	registryDir     = "/etc/containerd/certs.d/localhost:5001"
)

// Images contains the container images used by the local registry.
var Images = []string{registryImage}

// hostsTOML instructs containerd to route localhost:5001 pulls to the
// kind-registry container on the kind Docker network.
// IMPORTANT: use the container name (kind-registry), NOT localhost — inside
// node containers, localhost is the container's own loopback, not the host.
const hostsTOML = `[host."http://kind-registry:5000"]
`

type action struct{}

// NewAction returns a new action for installing the local registry.
func NewAction() actions.Action {
	return &action{}
}

// Execute runs the action.
func (a *action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Installing Local Registry")
	defer ctx.Status.End(false)

	// Detect provider binary name (docker, nerdctl, or podman).
	// Provider interface lacks String(); use fmt.Stringer type assertion
	// (same pattern as installmetallb).
	binaryName := "docker"
	if s, ok := ctx.Provider.(fmt.Stringer); ok {
		binaryName = s.String()
	}

	// Warn on rootless Podman — host-side push requires manual insecure
	// registry configuration that kinder cannot automate.
	info, err := ctx.Provider.Info()
	if err != nil {
		return errors.Wrap(err, "failed to get provider info")
	}
	if info.Rootless && binaryName == "podman" {
		ctx.Logger.Warn(
			"Rootless Podman detected: pushing to localhost:5001 requires manual insecure registry " +
				"configuration. Add to /etc/containers/registries.conf:\n" +
				"[[registry]]\n  location = \"localhost:5001\"\n  insecure = true\n" +
				"Then restart: podman machine stop && podman machine start",
		)
	}

	// Step 1 (REG-01): Create registry container (idempotent).
	// Check for existence first — the registry persists across cluster
	// recreations so cached images survive cluster restart.
	if exec.Command(binaryName, "inspect", "--format={{.ID}}", registryName).Run() != nil {
		if err := exec.Command(binaryName,
			"run", "-d", "--restart=always",
			"--name", registryName,
			"-p", "127.0.0.1:"+registryPort+":"+registryIntPort,
			registryImage,
		).Run(); err != nil {
			return errors.Wrap(err, "failed to create registry container")
		}
	}

	// Step 2 (REG-01): Connect registry to kind network (idempotent).
	// Check network membership before connecting — docker/podman/nerdctl all
	// return an error if the container is already connected.
	var networksOut strings.Builder
	_ = exec.Command(binaryName, "inspect",
		"--format", "{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}",
		registryName,
	).SetStdout(&networksOut).Run()
	if !strings.Contains(networksOut.String(), "kind") {
		if err := exec.Command(binaryName, "network", "connect", "kind", registryName).Run(); err != nil {
			return errors.Wrap(err, "failed to connect registry to kind network")
		}
	}

	// Step 3 (REG-02): Patch containerd certs.d on ALL nodes.
	// Each node is an independent container with its own containerd — ALL
	// nodes must be patched, not just control-plane.
	allNodes, err := ctx.Nodes()
	if err != nil {
		return errors.Wrap(err, "failed to list cluster nodes")
	}
	for _, node := range allNodes {
		if err := node.CommandContext(ctx.Context, "mkdir", "-p", registryDir).Run(); err != nil {
			return errors.Wrapf(err, "failed to create certs.d dir on node %s", node)
		}
		if err := node.CommandContext(ctx.Context, "tee", registryDir+"/hosts.toml").
			SetStdin(strings.NewReader(hostsTOML)).Run(); err != nil {
			return errors.Wrapf(err, "failed to write hosts.toml on node %s", node)
		}
	}

	// Step 4 (REG-03): Apply kube-public/local-registry-hosting ConfigMap.
	// Follows KEP-1755 so tools like Tilt and Skaffold auto-discover the
	// registry endpoint.
	controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
	if err != nil {
		return errors.Wrap(err, "failed to find control plane nodes")
	}
	if len(controlPlanes) == 0 {
		return errors.New("no control plane nodes found")
	}
	if err := controlPlanes[0].CommandContext(ctx.Context,
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"apply", "-f", "-",
	).SetStdin(strings.NewReader(localRegistryHostingManifest)).Run(); err != nil {
		return errors.Wrap(err, "failed to apply local-registry-hosting ConfigMap")
	}

	ctx.Status.End(true)
	ctx.Logger.V(0).Info("")
	ctx.Logger.V(0).Info("Local Registry:")
	ctx.Logger.V(0).Infof("  Push:        docker push localhost:%s/myimage:latest", registryPort)
	ctx.Logger.V(0).Infof("  Pull in pods: image: localhost:%s/myimage:latest", registryPort)
	ctx.Logger.V(0).Info("")

	return nil
}
