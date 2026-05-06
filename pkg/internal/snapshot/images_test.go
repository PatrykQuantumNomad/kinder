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
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// TestListImageRefs: fake ctr list -q returns 3 lines → returns 3 refs.
func TestListImageRefs(t *testing.T) {
	node := &captureCallbackNode{
		lookup: func(name string, args []string) (string, error) {
			if name == "ctr" {
				return "docker.io/library/pause:3.9\ndocker.io/library/etcd:3.5.1\ndocker.io/library/coredns:v1.10.0\n", nil
			}
			return "", nil
		},
	}

	refs, err := ListImageRefs(context.Background(), node)
	if err != nil {
		t.Fatalf("ListImageRefs returned error: %v", err)
	}
	if len(refs) != 3 {
		t.Errorf("expected 3 refs, got %d: %v", len(refs), refs)
	}
}

// TestCaptureImagesSuccess: refs returned; export succeeds; cat streams 2048 bytes → digest correct.
func TestCaptureImagesSuccess(t *testing.T) {
	knownBytes := make([]byte, 2048)
	for i := range knownBytes {
		knownBytes[i] = byte(i % 256)
	}
	expectedDigest := sha256.Sum256(knownBytes)
	expectedHex := hex.EncodeToString(expectedDigest[:])

	exportCalled := false
	node := &captureCallbackNode{
		lookup: func(name string, args []string) (string, error) {
			if name == "ctr" {
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "list") {
					return "image1\nimage2\n", nil
				}
				if strings.Contains(joined, "export") {
					exportCalled = true
					return "", nil
				}
			}
			if name == "cat" {
				return string(knownBytes), nil
			}
			if name == "rm" {
				return "", nil
			}
			return "", nil
		},
	}

	dstPath := t.TempDir() + "/images.tar"
	digest, err := CaptureImages(context.Background(), node, dstPath)
	if err != nil {
		t.Fatalf("CaptureImages returned error: %v", err)
	}
	if !exportCalled {
		t.Error("expected ctr export to be called when refs are present")
	}
	if digest != expectedHex {
		t.Errorf("digest = %q, want %q", digest, expectedHex)
	}
}

// TestCaptureImagesEmptyCluster: list returns empty → no export call; digest == sha256("").
func TestCaptureImagesEmptyCluster(t *testing.T) {
	emptyDigest := sha256.Sum256([]byte{})
	expectedHex := hex.EncodeToString(emptyDigest[:])

	exportCalled := false
	node := &captureCallbackNode{
		lookup: func(name string, args []string) (string, error) {
			if name == "ctr" {
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "list") {
					return "", nil // empty
				}
				if strings.Contains(joined, "export") {
					exportCalled = true
					return "", nil
				}
			}
			return "", nil
		},
	}

	dstPath := t.TempDir() + "/images.tar"
	digest, err := CaptureImages(context.Background(), node, dstPath)
	if err != nil {
		t.Fatalf("CaptureImages returned error: %v", err)
	}
	if exportCalled {
		t.Error("expected ctr export NOT to be called when no refs")
	}
	if digest != expectedHex {
		t.Errorf("digest = %q, want %q (sha256 of empty)", digest, expectedHex)
	}
}
