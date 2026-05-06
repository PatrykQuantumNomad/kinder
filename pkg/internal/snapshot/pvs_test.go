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
	"archive/tar"
	"context"
	"io"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// TestCapturePVsNoNodes: 2 fake nodes neither has the dir → empty file, digest "".
func TestCapturePVsNoNodes(t *testing.T) {
	node1 := &captureCallbackNode{
		name: "node1",
		lookup: func(name string, args []string) (string, error) {
			if name == "test" {
				// simulate directory does not exist
				return "", &fakeExitError{msg: "exit status 1"}
			}
			return "", nil
		},
	}
	node2 := &captureCallbackNode{
		name: "node2",
		lookup: func(name string, args []string) (string, error) {
			if name == "test" {
				return "", &fakeExitError{msg: "exit status 1"}
			}
			return "", nil
		},
	}

	dstPath := t.TempDir() + "/pvs.tar"
	digest, err := CapturePVs(context.Background(), []nodes.Node{node1, node2}, dstPath)
	if err != nil {
		t.Fatalf("CapturePVs returned error: %v", err)
	}
	if digest != "" {
		t.Errorf("expected empty digest when no nodes have LocalPathDir; got %q", digest)
	}
}

// TestCapturePVsOneNodeHasData: 2 nodes, only worker has dir; outer tar contains
// worker/local-path-provisioner.tar entry.
func TestCapturePVsOneNodeHasData(t *testing.T) {
	const workerTarContent = "fake-local-path-tar-content"

	node1 := &captureCallbackNode{
		name: "cp",
		lookup: func(name string, args []string) (string, error) {
			if name == "test" {
				return "", &fakeExitError{msg: "exit status 1"} // no dir
			}
			return "", nil
		},
	}
	node2 := &captureCallbackNode{
		name: "worker",
		lookup: func(name string, args []string) (string, error) {
			if name == "test" {
				return "", nil // dir exists
			}
			if name == "tar" {
				return workerTarContent, nil
			}
			return "", nil
		},
	}

	dstPath := t.TempDir() + "/pvs.tar"
	digest, err := CapturePVs(context.Background(), []nodes.Node{node1, node2}, dstPath)
	if err != nil {
		t.Fatalf("CapturePVs returned error: %v", err)
	}
	if digest == "" {
		t.Error("expected non-empty digest when worker has LocalPathDir")
	}

	// Verify outer tar has the expected entry
	entries := listTarEntries(t, dstPath)
	found := false
	for _, e := range entries {
		if strings.Contains(e, "worker") && strings.Contains(e, "local-path-provisioner") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected outer tar to have 'worker/local-path-provisioner.tar' entry; got: %v", entries)
	}
}

// TestCapturePVsMultipleNodes: 3 nodes all have data; outer tar has 3 entries.
func TestCapturePVsMultipleNodes(t *testing.T) {
	makeLookup := func(content string) func(string, []string) (string, error) {
		return func(name string, args []string) (string, error) {
			if name == "test" {
				return "", nil // dir exists
			}
			if name == "tar" {
				return content, nil
			}
			return "", nil
		}
	}

	node1 := &captureCallbackNode{name: "cp1", lookup: makeLookup("cp1-pvs")}
	node2 := &captureCallbackNode{name: "worker1", lookup: makeLookup("worker1-pvs")}
	node3 := &captureCallbackNode{name: "worker2", lookup: makeLookup("worker2-pvs")}

	dstPath := t.TempDir() + "/pvs.tar"
	digest, err := CapturePVs(context.Background(), []nodes.Node{node1, node2, node3}, dstPath)
	if err != nil {
		t.Fatalf("CapturePVs returned error: %v", err)
	}
	if digest == "" {
		t.Error("expected non-empty digest when all nodes have LocalPathDir")
	}

	entries := listTarEntries(t, dstPath)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries in outer tar; got %d: %v", len(entries), entries)
	}
}

// listTarEntries opens a tar file and returns the names of all entries.
func listTarEntries(t *testing.T, path string) []string {
	t.Helper()
	f, err := openFileForRead(path)
	if err != nil {
		t.Fatalf("open tar: %v", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		names = append(names, hdr.Name)
	}
	return names
}
