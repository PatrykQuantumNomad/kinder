/*
Copyright 2026 The Kubernetes Authors.

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

package snapshot

import (
	"context"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// --- ClassifyFn helpers for tests ---

// makeClassifyFn builds a ClassifyFn that returns a fixed partition.
func makeClassifyFn(cp, workers []nodes.Node, lb nodes.Node) ClassifyFn {
	return func(allNodes []nodes.Node) ([]nodes.Node, []nodes.Node, nodes.Node, error) {
		return cp, workers, lb, nil
	}
}

// --- CaptureTopology tests ---

// TestCaptureTopologySingleCP: 1 CP, no workers, no LB.
func TestCaptureTopologySingleCP(t *testing.T) {
	const k8sVer = "v1.31.2"
	const nodeImg = "kindest/node:v1.31.2"

	cp := &captureCallbackNode{
		name: "kind-control-plane",
		role: "control-plane",
		lookup: func(name string, args []string) (string, error) {
			if name == "cat" {
				return k8sVer + "\n", nil
			}
			return "", nil
		},
	}

	// providerBin is passed for potential image inspection; not used in test since
	// we don't execute real docker commands. Pass a fake binary name.
	// For the test the node image comes from the providerBin inspect call.
	// We fake by returning the image from the lookup.
	// In the real implementation CaptureTopology calls
	// `<providerBin> inspect --format {{.Config.Image}} <nodeName>`,
	// but we route node.Command through our fake.
	cpNode := &captureCallbackNode{
		name: "kind-control-plane",
		role: "control-plane",
		lookup: func(name string, args []string) (string, error) {
			// cat /kind/version
			if name == "cat" {
				return k8sVer + "\n", nil
			}
			// providerBin inspect: return node image
			if strings.Contains(strings.Join(args, " "), "inspect") {
				return nodeImg + "\n", nil
			}
			return "", nil
		},
	}
	_ = cp

	classify := makeClassifyFn(
		[]nodes.Node{cpNode}, // cp
		nil,                  // workers
		nil,                  // lb
	)

	topo, gotVersion, gotImage, err := CaptureTopology(context.Background(), []nodes.Node{cpNode}, classify, "docker")
	if err != nil {
		t.Fatalf("CaptureTopology returned error: %v", err)
	}
	if topo.ControlPlaneCount != 1 {
		t.Errorf("ControlPlaneCount = %d, want 1", topo.ControlPlaneCount)
	}
	if topo.WorkerCount != 0 {
		t.Errorf("WorkerCount = %d, want 0", topo.WorkerCount)
	}
	if topo.HasLoadBalancer {
		t.Errorf("HasLoadBalancer = true, want false")
	}
	if gotVersion != k8sVer {
		t.Errorf("k8sVersion = %q, want %q", gotVersion, k8sVer)
	}
	if gotImage != nodeImg {
		t.Errorf("nodeImage = %q, want %q", gotImage, nodeImg)
	}
}

// TestCaptureTopologyHA: 3 CP + 2 workers + 1 lb → counts correct, HasLoadBalancer true.
func TestCaptureTopologyHA(t *testing.T) {
	const k8sVer = "v1.31.2"
	const nodeImg = "kindest/node:v1.31.2"

	makeCPNode := func(name string) *captureCallbackNode {
		return &captureCallbackNode{
			name: name,
			role: "control-plane",
			lookup: func(n string, args []string) (string, error) {
				if n == "cat" {
					return k8sVer + "\n", nil
				}
				if strings.Contains(strings.Join(args, " "), "inspect") {
					return nodeImg + "\n", nil
				}
				return "", nil
			},
		}
	}
	makeWorker := func(name string) *captureCallbackNode {
		return &captureCallbackNode{name: name, role: "worker"}
	}
	makeLB := func(name string) *captureCallbackNode {
		return &captureCallbackNode{name: name, role: "external-load-balancer"}
	}

	cp1 := makeCPNode("cp1")
	cp2 := makeCPNode("cp2")
	cp3 := makeCPNode("cp3")
	w1 := makeWorker("worker1")
	w2 := makeWorker("worker2")
	lb := makeLB("lb")

	allNodes := []nodes.Node{cp1, cp2, cp3, w1, w2, lb}
	classify := makeClassifyFn(
		[]nodes.Node{cp1, cp2, cp3},
		[]nodes.Node{w1, w2},
		lb,
	)

	topo, gotVersion, _, err := CaptureTopology(context.Background(), allNodes, classify, "docker")
	if err != nil {
		t.Fatalf("CaptureTopology returned error: %v", err)
	}
	if topo.ControlPlaneCount != 3 {
		t.Errorf("ControlPlaneCount = %d, want 3", topo.ControlPlaneCount)
	}
	if topo.WorkerCount != 2 {
		t.Errorf("WorkerCount = %d, want 2", topo.WorkerCount)
	}
	if !topo.HasLoadBalancer {
		t.Errorf("HasLoadBalancer = false, want true")
	}
	if gotVersion != k8sVer {
		t.Errorf("k8sVersion = %q, want %q", gotVersion, k8sVer)
	}
}

// --- CaptureAddonVersions tests ---

// TestCaptureAddonVersionsAllPresent: fake kubectl returns image strings for 6
// of 7 probes; one returns NotFound; result map has 6 entries.
func TestCaptureAddonVersionsAllPresent(t *testing.T) {
	// Build lookup: for each known addon return an image with a tag, except one
	// returns "Error from server (NotFound)".
	installedAddons := map[string]string{
		"local-path-provisioner": "rancher/local-path-provisioner:v0.0.24",
		"controller":             "quay.io/metallb/controller:v0.13.7",
		"metrics-server":         "registry.k8s.io/metrics-server/metrics-server:v0.6.3",
		"kubernetes-dashboard":   "kubernetesui/dashboard:v2.7.0",
		"cert-manager":           "quay.io/jetstack/cert-manager-controller:v1.13.0",
		"envoy-gateway":          "envoyproxy/gateway:v0.5.0",
		// coredns — returns NotFound (not installed as a separate deployment)
	}
	node := &captureCallbackNode{
		name: "cp",
		role: "control-plane",
		lookup: func(name string, args []string) (string, error) {
			if name == "kubectl" {
				// Extract deployment name from args (last non-flag arg before -o jsonpath)
				for _, a := range args {
					if img, ok := installedAddons[a]; ok {
						return img + "\n", nil
					}
				}
				// Not found
				return "Error from server (NotFound): deployments.apps not found", &fakeExitError{msg: "exit status 1"}
			}
			return "", nil
		},
	}

	result, err := CaptureAddonVersions(context.Background(), node)
	if err != nil {
		t.Fatalf("CaptureAddonVersions returned error: %v", err)
	}
	if len(result) != 6 {
		t.Errorf("expected 6 addon entries, got %d: %v", len(result), result)
	}
	// All values should be non-empty (parsed tag or full image)
	for k, v := range result {
		if v == "" {
			t.Errorf("addon %q has empty version in result", k)
		}
	}
}

// TestCaptureAddonVersionsNoAddons: every probe returns NotFound → empty (non-nil) map.
func TestCaptureAddonVersionsNoAddons(t *testing.T) {
	node := &captureCallbackNode{
		name: "cp",
		role: "control-plane",
		lookup: func(name string, args []string) (string, error) {
			if name == "kubectl" {
				return "Error from server (NotFound): deployments.apps not found", &fakeExitError{msg: "exit status 1"}
			}
			return "", nil
		},
	}

	result, err := CaptureAddonVersions(context.Background(), node)
	if err != nil {
		t.Fatalf("CaptureAddonVersions returned error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil (empty) map when no addons installed")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map when no addons installed; got %v", result)
	}
}
