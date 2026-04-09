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

package doctor

import (
	"bytes"
	"fmt"
	"os/exec"
	osexec "os/exec"
	"text/tabwriter"
)

// addonImage pairs a container image reference with the addon name it belongs to.
type addonImage struct {
	Image string
	Addon string // human-readable addon name for output
}

// allAddonImages is the canonical list of addon images required by kinder addons.
// Tags are sourced from the embedded manifests and const files — do NOT edit without
// also updating the corresponding manifest or const.
//
// Image counts by addon:
//   - Load Balancer (HA):  1
//   - Local Registry:      1
//   - MetalLB:             2
//   - Metrics Server:      1
//   - Cert Manager:        3
//   - Envoy Gateway:       2
//   - Dashboard:           1
//   - NVIDIA GPU:          1
//
// Total: 12
var allAddonImages = []addonImage{
	// Load Balancer (HA clusters only)
	{"docker.io/kindest/haproxy:v20260131-7181c60a", "Load Balancer (HA)"},
	// Local Registry
	{"registry:2", "Local Registry"},
	// MetalLB
	{"quay.io/metallb/controller:v0.15.3", "MetalLB"},
	{"quay.io/metallb/speaker:v0.15.3", "MetalLB"},
	// Metrics Server
	{"registry.k8s.io/metrics-server/metrics-server:v0.8.1", "Metrics Server"},
	// Cert Manager
	{"quay.io/jetstack/cert-manager-cainjector:v1.16.3", "Cert Manager"},
	{"quay.io/jetstack/cert-manager-controller:v1.16.3", "Cert Manager"},
	{"quay.io/jetstack/cert-manager-webhook:v1.16.3", "Cert Manager"},
	// Envoy Gateway
	{"docker.io/envoyproxy/ratelimit:ae4cee11", "Envoy Gateway"},
	{"envoyproxy/gateway:v1.3.1", "Envoy Gateway"},
	// Dashboard
	{"ghcr.io/headlamp-k8s/headlamp:v0.40.1", "Dashboard"},
	// NVIDIA GPU
	{"nvcr.io/nvidia/k8s-device-plugin:v0.17.1", "NVIDIA GPU"},
}

// offlineReadinessCheck reports which addon images are absent from the local
// image store.  It is intended as a pre-flight check before air-gapped cluster
// creation so users can identify and pre-load missing images.
type offlineReadinessCheck struct {
	// inspectImage returns true when the given image is present in the local store.
	// Injected for unit testing; defaults to realInspectImage.
	inspectImage func(image string) bool
	// lookPath is injected for testing; defaults to osexec.LookPath.
	lookPath func(string) (string, error)
}

// newOfflineReadinessCheck creates an offlineReadinessCheck with real system deps.
func newOfflineReadinessCheck() Check {
	return &offlineReadinessCheck{
		inspectImage: realInspectImage,
		lookPath:     osexec.LookPath,
	}
}

func (c *offlineReadinessCheck) Name() string       { return "offline-readiness" }
func (c *offlineReadinessCheck) Category() string    { return "Offline" }
func (c *offlineReadinessCheck) Platforms() []string { return nil } // all platforms

// Run executes the offline-readiness check:
//  1. If no container runtime is available → skip.
//  2. Check each addon image with inspectImage.
//  3. If all present → ok.
//  4. If any absent → warn with a table of missing images.
func (c *offlineReadinessCheck) Run() []Result {
	// Check whether any container runtime is available.
	hasRuntime := false
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := c.lookPath(rt); err == nil {
			hasRuntime = true
			break
		}
	}
	if !hasRuntime {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "no container runtime found",
		}}
	}

	// Inspect each addon image.
	var absent []addonImage
	for _, entry := range allAddonImages {
		if !c.inspectImage(entry.Image) {
			absent = append(absent, entry)
		}
	}

	if len(absent) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  fmt.Sprintf("all %d addon images present locally", len(allAddonImages)),
		}}
	}

	// Build a tab-aligned table of missing images.
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MISSING IMAGE\tREQUIRED BY")
	fmt.Fprintln(w, "-------------\t-----------")
	for _, m := range absent {
		fmt.Fprintf(w, "%s\t%s\n", m.Image, m.Addon)
	}
	w.Flush() //nolint:errcheck

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "warn",
		Message:  fmt.Sprintf("%d of %d addon images missing:\n%s", len(absent), len(allAddonImages), buf.String()),
		Reason:   "Air-gapped cluster creation (--air-gapped) will fail if these images are not pre-loaded",
		Fix:      "Pre-load missing images: docker pull <image> (or podman/nerdctl). See: kinder create cluster --help",
	}}
}

// realInspectImage returns true when the given image is present in the local
// image store of the first available container runtime (docker, podman, nerdctl).
// Returns false when no runtime is found or the image is absent.
func realInspectImage(image string) bool {
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := osexec.LookPath(rt); err == nil {
			cmd := exec.Command(rt, "inspect", "--type=image", image)
			return cmd.Run() == nil
		}
	}
	return false // no runtime found
}
