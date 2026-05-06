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
	"fmt"
	"os"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// ---------------------------------------------------------------------------
// TestRestoreImagesUsesInjectedFn
// ---------------------------------------------------------------------------

// TestRestoreImagesUsesInjectedFn verifies that RestoreImages calls the
// injected importFn exactly once, passing allNodes and imagesTarHostPath.
func TestRestoreImagesUsesInjectedFn(t *testing.T) {
	// Create a temp file as the "images.tar"
	f, err := os.CreateTemp(t.TempDir(), "images-*.tar")
	if err != nil {
		t.Fatalf("create temp tar: %v", err)
	}
	f.WriteString("fake-tar-content")
	f.Close()
	tarPath := f.Name()

	allNodes := []nodes.Node{
		&snapFakeNode{name: "node1", role: "worker"},
		&snapFakeNode{name: "node2", role: "control-plane"},
	}

	var calledNodes []nodes.Node
	var calledPath string
	var callCount int

	fakeFn := func(ctx context.Context, nodes []nodes.Node, archivePath string) error {
		callCount++
		calledNodes = nodes
		calledPath = archivePath
		return nil
	}

	if err := RestoreImages(context.Background(), allNodes, tarPath, fakeFn); err != nil {
		t.Fatalf("RestoreImages returned error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("importFn called %d times, want 1", callCount)
	}
	if len(calledNodes) != len(allNodes) {
		t.Errorf("importFn received %d nodes, want %d", len(calledNodes), len(allNodes))
	}
	if calledPath != tarPath {
		t.Errorf("importFn called with path %q, want %q", calledPath, tarPath)
	}
}

// ---------------------------------------------------------------------------
// TestRestoreImagesEmptyDigest
// ---------------------------------------------------------------------------

// TestRestoreImagesEmptyDigest verifies that when the images.tar file has zero
// bytes, the importFn is NOT invoked and nil is returned (defense-in-depth
// empty-file gate, in addition to Plan 04's metadata.ImagesDigest check).
func TestRestoreImagesEmptyDigest(t *testing.T) {
	// Create a 0-byte file.
	f, err := os.CreateTemp(t.TempDir(), "empty-images-*.tar")
	if err != nil {
		t.Fatalf("create empty temp file: %v", err)
	}
	f.Close()
	emptyPath := f.Name()

	var importFnCalled bool
	fakeFn := func(ctx context.Context, nodes []nodes.Node, archivePath string) error {
		importFnCalled = true
		return fmt.Errorf("should not have been called")
	}

	allNodes := []nodes.Node{&snapFakeNode{name: "node1", role: "worker"}}

	if err := RestoreImages(context.Background(), allNodes, emptyPath, fakeFn); err != nil {
		t.Fatalf("RestoreImages on empty file returned error: %v", err)
	}
	if importFnCalled {
		t.Error("importFn was invoked on a 0-byte images.tar (should be skipped)")
	}
}

// ---------------------------------------------------------------------------
// TestRestoreImagesPropagatesError
// ---------------------------------------------------------------------------

// TestRestoreImagesPropagatesError verifies that when the importFn returns an
// error, RestoreImages wraps and returns it (does not swallow the error).
func TestRestoreImagesPropagatesError(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "images-*.tar")
	if err != nil {
		t.Fatalf("create temp tar: %v", err)
	}
	f.WriteString("fake-tar-content-for-error-test")
	f.Close()
	tarPath := f.Name()

	sentinel := fmt.Errorf("simulated import failure")
	fakeFn := func(ctx context.Context, nodes []nodes.Node, archivePath string) error {
		return sentinel
	}

	allNodes := []nodes.Node{&snapFakeNode{name: "node1", role: "worker"}}

	err = RestoreImages(context.Background(), allNodes, tarPath, fakeFn)
	if err == nil {
		t.Fatal("RestoreImages should have returned an error when importFn fails; got nil")
	}
	// Error must wrap the original message.
	if !contains(err.Error(), "restore images") && !contains(err.Error(), "simulated import failure") {
		t.Errorf("error should contain 'restore images' or original error text; got: %v", err)
	}
}

// contains is a helper to check substring presence.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
