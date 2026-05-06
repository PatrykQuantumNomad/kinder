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

package dev

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// --- Fakes for LoadImagesIntoCluster ---

// loadFakeNode is a minimal nodes.Node — only String() is used by load.go;
// Command/CommandContext are routed through devCmder so the fake exec.Cmder
// captures any unexpected node-side invocations.
type loadFakeNode struct {
	name string
}

var _ nodes.Node = (*loadFakeNode)(nil)

func (n *loadFakeNode) String() string                      { return n.name }
func (n *loadFakeNode) Role() (string, error)               { return "worker", nil }
func (n *loadFakeNode) IP() (string, string, error)         { return "", "", nil }
func (n *loadFakeNode) SerialLogs(_ io.Writer) error        { return nil }
func (n *loadFakeNode) Command(c string, a ...string) exec.Cmd {
	return devCmder.Command(c, a...)
}
func (n *loadFakeNode) CommandContext(ctx context.Context, c string, a ...string) exec.Cmd {
	return devCmder.CommandContext(ctx, c, a...)
}

// withNodeLister swaps the package-level nodeLister for the duration of t.
func withNodeLister(t *testing.T, fn func(clusterName string) ([]nodes.Node, error)) {
	t.Helper()
	prev := nodeLister
	nodeLister = fn
	t.Cleanup(func() { nodeLister = prev })
}

// withImageTagsFn swaps the package-level imageTagsFn for the duration of t.
func withImageTagsFn(t *testing.T, fn func(n nodes.Node, imageID string) (map[string]bool, error)) {
	t.Helper()
	prev := imageTagsFn
	imageTagsFn = fn
	t.Cleanup(func() { imageTagsFn = prev })
}

// withReTagFn swaps the package-level reTagFn for the duration of t.
func withReTagFn(t *testing.T, fn func(n nodes.Node, imageID, imageName string) error) {
	t.Helper()
	prev := reTagFn
	reTagFn = fn
	t.Cleanup(func() { reTagFn = prev })
}

// withImageInspectID swaps the package-level imageInspectID for the duration
// of t.
func withImageInspectID(t *testing.T, fn func(ctx context.Context, binaryName, ref string) (string, error)) {
	t.Helper()
	prev := imageInspectID
	imageInspectID = fn
	t.Cleanup(func() { imageInspectID = prev })
}

// recordingLoader is an ImageLoaderFn that records every (node, openTar)
// invocation and returns a per-node error from a programmable map.
type recordingLoader struct {
	mu       sync.Mutex
	loaded   []string
	errByNode map[string]error
}

func (r *recordingLoader) load(n nodes.Node, _ func() (io.ReadCloser, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded = append(r.loaded, n.String())
	if r.errByNode != nil {
		if err, ok := r.errByNode[n.String()]; ok {
			return err
		}
	}
	return nil
}

func (r *recordingLoader) loadedSnapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.loaded))
	copy(out, r.loaded)
	return out
}

// stubExecCmderForLoad returns a fakeExecCmder that succeeds for any call —
// used for the host-side `<binary> save` invocation.
func stubExecCmderForLoad() *fakeExecCmder {
	return &fakeExecCmder{lookup: func(_ string, _ []string) (string, error) {
		return "", nil
	}}
}

// --- LoadImagesIntoCluster tests ---

func TestLoadImagesIntoCluster_RejectsEmptyImageTag(t *testing.T) {
	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err == nil {
		t.Fatal("expected error for empty ImageTag")
	}
	if !strings.Contains(err.Error(), "ImageTag") {
		t.Errorf("error %q should mention ImageTag", err.Error())
	}
}

func TestLoadImagesIntoCluster_RejectsEmptyClusterName(t *testing.T) {
	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err == nil {
		t.Fatal("expected error for empty ClusterName")
	}
	if !strings.Contains(err.Error(), "ClusterName") {
		t.Errorf("error %q should mention ClusterName", err.Error())
	}
}

func TestLoadImagesIntoCluster_RejectsEmptyBinaryName(t *testing.T) {
	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err == nil {
		t.Fatal("expected error for empty BinaryName")
	}
	if !strings.Contains(err.Error(), "BinaryName") {
		t.Errorf("error %q should mention BinaryName", err.Error())
	}
}

// TestLoadImagesIntoCluster_SmartSkipPresent: every node already has the
// image with the correct tag → loader is NEVER called.
func TestLoadImagesIntoCluster_SmartSkipPresent(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	nodesA := []nodes.Node{&loadFakeNode{name: "node-a"}, &loadFakeNode{name: "node-b"}}
	withNodeLister(t, func(_ string) ([]nodes.Node, error) { return nodesA, nil })
	withImageTagsFn(t, func(_ nodes.Node, _ string) (map[string]bool, error) {
		return map[string]bool{"img:tag": true}, nil
	})
	withReTagFn(t, func(_ nodes.Node, _, _ string) error {
		t.Errorf("ReTag should not be called when tag is already correct on every node")
		return nil
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err != nil {
		t.Fatalf("LoadImagesIntoCluster returned error: %v", err)
	}
	if got := loader.loadedSnapshot(); len(got) != 0 {
		t.Errorf("expected zero loader calls in smart-skip path; got %v", got)
	}
}

// TestLoadImagesIntoCluster_LoadsToMissingNodes: 3 nodes; node-a has the image
// with the right tag, node-b and node-c don't (empty tags map) → loader called
// for exactly node-b and node-c.
func TestLoadImagesIntoCluster_LoadsToMissingNodes(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	nodesA := []nodes.Node{
		&loadFakeNode{name: "node-a"},
		&loadFakeNode{name: "node-b"},
		&loadFakeNode{name: "node-c"},
	}
	withNodeLister(t, func(_ string) ([]nodes.Node, error) { return nodesA, nil })
	withImageTagsFn(t, func(n nodes.Node, _ string) (map[string]bool, error) {
		if n.String() == "node-a" {
			return map[string]bool{"img:tag": true}, nil
		}
		return map[string]bool{}, nil // image not present
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err != nil {
		t.Fatalf("LoadImagesIntoCluster returned error: %v", err)
	}
	got := loader.loadedSnapshot()
	if len(got) != 2 {
		t.Fatalf("expected loader called for 2 nodes; got %v", got)
	}
	loadedSet := map[string]bool{}
	for _, n := range got {
		loadedSet[n] = true
	}
	if !loadedSet["node-b"] || !loadedSet["node-c"] {
		t.Errorf("expected loader called for node-b and node-c; got %v", got)
	}
	if loadedSet["node-a"] {
		t.Errorf("loader must NOT be called for node-a (smart-skip target); got %v", got)
	}
}

// TestLoadImagesIntoCluster_ReTagWhenWrongTag: image exists by ID but under a
// different tag → reTagFn is called, loader is NOT called for that node.
func TestLoadImagesIntoCluster_ReTagWhenWrongTag(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	nodesA := []nodes.Node{&loadFakeNode{name: "node-a"}}
	withNodeLister(t, func(_ string) ([]nodes.Node, error) { return nodesA, nil })
	withImageTagsFn(t, func(_ nodes.Node, _ string) (map[string]bool, error) {
		// image present under a different tag
		return map[string]bool{"otherimg:other": true}, nil
	})

	var reTagCalled int
	var reTagArgs struct {
		imageID, imageName string
	}
	withReTagFn(t, func(_ nodes.Node, imageID, imageName string) error {
		reTagCalled++
		reTagArgs.imageID = imageID
		reTagArgs.imageName = imageName
		return nil
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err != nil {
		t.Fatalf("LoadImagesIntoCluster returned error: %v", err)
	}
	if reTagCalled != 1 {
		t.Errorf("expected reTagFn called exactly once; got %d", reTagCalled)
	}
	if reTagArgs.imageID != "sha256:imageid" || reTagArgs.imageName != "img:tag" {
		t.Errorf("reTag args = (%q, %q), want (sha256:imageid, img:tag)", reTagArgs.imageID, reTagArgs.imageName)
	}
	if got := loader.loadedSnapshot(); len(got) != 0 {
		t.Errorf("expected loader NOT called when retag succeeds; got %v", got)
	}
}

// TestLoadImagesIntoCluster_ReTagFallbackOnFailure: reTagFn fails for a node
// → loader IS called for that node (fallback).
func TestLoadImagesIntoCluster_ReTagFallbackOnFailure(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	nodesA := []nodes.Node{&loadFakeNode{name: "node-a"}}
	withNodeLister(t, func(_ string) ([]nodes.Node, error) { return nodesA, nil })
	withImageTagsFn(t, func(_ nodes.Node, _ string) (map[string]bool, error) {
		return map[string]bool{"otherimg:other": true}, nil
	})
	withReTagFn(t, func(_ nodes.Node, _, _ string) error {
		return errors.New("retag failed")
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err != nil {
		t.Fatalf("LoadImagesIntoCluster returned error: %v", err)
	}
	got := loader.loadedSnapshot()
	if len(got) != 1 || got[0] != "node-a" {
		t.Errorf("expected loader called for node-a (retag-fallback); got %v", got)
	}
}

// TestLoadImagesIntoCluster_ImageInspectFails: imageInspectID returns error
// → wrapped error containing "inspect image"; nodes never queried.
func TestLoadImagesIntoCluster_ImageInspectFails(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "", errors.New("inspect boom")
	})
	nodeListerCalled := false
	withNodeLister(t, func(_ string) ([]nodes.Node, error) {
		nodeListerCalled = true
		return nil, nil
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err == nil {
		t.Fatal("expected error from imageInspectID failure")
	}
	if !strings.Contains(err.Error(), "inspect image") {
		t.Errorf("error %q should contain 'inspect image' for context", err.Error())
	}
	if !strings.Contains(err.Error(), "inspect boom") {
		t.Errorf("error %q should propagate underlying message", err.Error())
	}
	if nodeListerCalled {
		t.Errorf("nodeLister must not be called when imageInspectID fails")
	}
}

// TestLoadImagesIntoCluster_ListNodesError: provider.ListInternalNodes
// returns error → LoadImagesIntoCluster returns wrapped error.
func TestLoadImagesIntoCluster_ListNodesError(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	withNodeLister(t, func(_ string) ([]nodes.Node, error) {
		return nil, errors.New("list nodes boom")
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err == nil {
		t.Fatal("expected error from nodeLister failure")
	}
	if !strings.Contains(err.Error(), "list nodes") {
		t.Errorf("error %q should mention 'list nodes'", err.Error())
	}
	if !strings.Contains(err.Error(), "list nodes boom") {
		t.Errorf("error %q should propagate underlying message", err.Error())
	}
}

// TestLoadImagesIntoCluster_LoaderErrorPropagated: loader returns error for
// one node → final error references that error.
func TestLoadImagesIntoCluster_LoaderErrorPropagated(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	nodesA := []nodes.Node{&loadFakeNode{name: "node-a"}, &loadFakeNode{name: "node-b"}}
	withNodeLister(t, func(_ string) ([]nodes.Node, error) { return nodesA, nil })
	withImageTagsFn(t, func(_ nodes.Node, _ string) (map[string]bool, error) {
		return map[string]bool{}, nil // both missing
	})

	loader := &recordingLoader{
		errByNode: map[string]error{"node-b": errors.New("import-pipe-broken")},
	}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err == nil {
		t.Fatal("expected aggregated error from loader failure")
	}
	if !strings.Contains(err.Error(), "import-pipe-broken") {
		t.Errorf("error %q should propagate underlying loader error", err.Error())
	}
}

// TestLoadImagesIntoCluster_TagsErrorTreatsNodeAsCandidate: imageTagsFn
// returns error for a node (e.g. crictl unreachable) → that node becomes a
// load candidate (defensive fallback, matches kinder load images behavior).
func TestLoadImagesIntoCluster_TagsErrorTreatsNodeAsCandidate(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	nodesA := []nodes.Node{&loadFakeNode{name: "node-a"}}
	withNodeLister(t, func(_ string) ([]nodes.Node, error) { return nodesA, nil })
	withImageTagsFn(t, func(_ nodes.Node, _ string) (map[string]bool, error) {
		return nil, errors.New("crictl unreachable")
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err != nil {
		t.Fatalf("LoadImagesIntoCluster returned error: %v", err)
	}
	got := loader.loadedSnapshot()
	if len(got) != 1 || got[0] != "node-a" {
		t.Errorf("expected loader called for node-a (tags-error fallback); got %v", got)
	}
}

// TestLoadImagesIntoCluster_NoNodes: cluster has zero internal nodes →
// loader never called, no error (pipeline bails after empty candidate list).
func TestLoadImagesIntoCluster_NoNodes(t *testing.T) {
	withDevCmder(t, stubExecCmderForLoad())
	withImageInspectID(t, func(_ context.Context, _, _ string) (string, error) {
		return "sha256:imageid", nil
	})
	withNodeLister(t, func(_ string) ([]nodes.Node, error) {
		return []nodes.Node{}, nil
	})

	loader := &recordingLoader{}
	err := LoadImagesIntoCluster(context.Background(), LoadOptions{
		ClusterName:   "kind",
		ImageTag:      "img:tag",
		BinaryName:    "docker",
		Logger:        log.NoopLogger{},
		ImageLoaderFn: loader.load,
	})
	if err != nil {
		t.Fatalf("LoadImagesIntoCluster returned error: %v", err)
	}
	if got := loader.loadedSnapshot(); len(got) != 0 {
		t.Errorf("expected zero loader calls when cluster has no nodes; got %v", got)
	}
}
