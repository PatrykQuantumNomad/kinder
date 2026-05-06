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
	"os"
	"strings"
	"testing"
)

// TestCaptureEtcdSuccess: fake crictl ps returns "abc123\n", fake crictl exec
// etcdctl snapshot save succeeds, fake cat writes 1024 known bytes; assert
// digest == sha256(known bytes).
func TestCaptureEtcdSuccess(t *testing.T) {
	const containerID = "abc123"
	knownBytes := make([]byte, 1024)
	for i := range knownBytes {
		knownBytes[i] = byte(i % 256)
	}
	expectedDigest := sha256.Sum256(knownBytes)
	expectedHex := hex.EncodeToString(expectedDigest[:])

	node := &captureCallbackNode{
		lookup: func(name string, args []string) (string, error) {
			if name == "crictl" && len(args) > 0 && args[0] == "ps" {
				return containerID + "\n", nil
			}
			if name == "crictl" && len(args) > 0 && args[0] == "exec" {
				// snapshot save call — check it has etcdctl and snapshot save
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "snapshot") && strings.Contains(joined, "save") {
					return "", nil
				}
				return "", nil
			}
			if name == "cat" {
				// Stream the known bytes back
				return string(knownBytes), nil
			}
			if name == "rm" {
				return "", nil
			}
			return "", nil
		},
	}

	dstPath := t.TempDir() + "/etcd.snap"
	digest, err := CaptureEtcd(context.Background(), node, dstPath)
	if err != nil {
		t.Fatalf("CaptureEtcd returned unexpected error: %v", err)
	}
	if digest != expectedHex {
		t.Errorf("digest = %q, want %q", digest, expectedHex)
	}

	// Verify the file was written
	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("expected dstPath to exist: %v", err)
	}
	if string(data) != string(knownBytes) {
		t.Errorf("file contents do not match known bytes")
	}
}

// TestCaptureEtcdNoEtcdContainer: fake crictl ps returns "" → returns
// descriptive error.
func TestCaptureEtcdNoEtcdContainer(t *testing.T) {
	node := &captureCallbackNode{
		lookup: func(name string, args []string) (string, error) {
			if name == "crictl" && len(args) > 0 && args[0] == "ps" {
				return "", nil // empty — no etcd container
			}
			return "", nil
		},
	}

	dstPath := t.TempDir() + "/etcd.snap"
	_, err := CaptureEtcd(context.Background(), node, dstPath)
	if err == nil {
		t.Fatal("expected error when no etcd container running, got nil")
	}
	if !strings.Contains(err.Error(), "etcd") {
		t.Errorf("error should mention etcd; got: %v", err)
	}
}

// TestCaptureEtcdSnapshotFails: fake crictl exec etcdctl snapshot save exits 1
// → error wrapped with "etcdctl snapshot save".
func TestCaptureEtcdSnapshotFails(t *testing.T) {
	const containerID = "etcdcid"
	node := &captureCallbackNode{
		lookup: func(name string, args []string) (string, error) {
			if name == "crictl" && len(args) > 0 && args[0] == "ps" {
				return containerID + "\n", nil
			}
			if name == "crictl" && len(args) > 0 && args[0] == "exec" {
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "snapshot") && strings.Contains(joined, "save") {
					return "", &fakeExitError{msg: "exit status 1"}
				}
				return "", nil
			}
			return "", nil
		},
	}

	dstPath := t.TempDir() + "/etcd.snap"
	_, err := CaptureEtcd(context.Background(), node, dstPath)
	if err == nil {
		t.Fatal("expected error when etcdctl snapshot save fails, got nil")
	}
	if !strings.Contains(err.Error(), "etcdctl snapshot save") {
		t.Errorf("error should mention 'etcdctl snapshot save'; got: %v", err)
	}
}
