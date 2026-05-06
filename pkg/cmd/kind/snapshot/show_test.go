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
	"encoding/json"
	"strings"
	"testing"
	"time"

	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"
)

// withShowFn swaps the package-level showFn for the duration of t.
func withShowFn(t *testing.T, fn func(ctx context.Context, root, clusterName, snapName string) (*showResult, error)) {
	t.Helper()
	prev := showFn
	showFn = fn
	t.Cleanup(func() { showFn = prev })
}

// fakeShowResult returns a non-nil showResult for use in tests.
func fakeShowResult() *showResult {
	return &showResult{
		Meta: &snapshot.Metadata{
			SchemaVersion: "1",
			Name:          "snap-20260101-120000",
			ClusterName:   "mycluster",
			CreatedAt:     time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			K8sVersion:    "v1.31.2",
			NodeImage:     "kindest/node:v1.31.2",
			Topology: snapshot.TopologyInfo{
				ControlPlaneCount: 1,
				WorkerCount:       2,
				HasLoadBalancer:   true,
			},
			AddonVersions: map[string]string{
				"calico":         "v3.28.0",
				"metrics-server": "v0.7.1",
			},
			EtcdDigest:    "abc123",
			ImagesDigest:  "def456",
			PVsDigest:     "ghi789",
			ArchiveDigest: "jkl012",
		},
		Size: 512 * 1024 * 1024, // 512 MiB
	}
}

// TestShowVertical: injected showFn returns metadata; output contains key fields.
func TestShowVertical(t *testing.T) {
	withShowFn(t, func(_ context.Context, _, _, _ string) (*showResult, error) {
		return fakeShowResult(), nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"show", "mycluster", "snap-20260101-120000"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	out := stdout.String()

	for _, expected := range []string{"Name:", "Cluster:", "K8s Version:", "Topology:", "Addons:", "Digests:"} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q in vertical output; got:\n%s", expected, out)
		}
	}
	// Check actual values appear.
	if !strings.Contains(out, "snap-20260101-120000") {
		t.Errorf("expected snap name in output; got:\n%s", out)
	}
	if !strings.Contains(out, "v1.31.2") {
		t.Errorf("expected k8s version in output; got:\n%s", out)
	}
}

// TestShowJSON: --output=json → valid JSON with Metadata fields.
func TestShowJSON(t *testing.T) {
	withShowFn(t, func(_ context.Context, _, _, _ string) (*showResult, error) {
		return fakeShowResult(), nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"show", "mycluster", "snap-20260101-120000", "--output=json"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	for _, k := range []string{"name", "clusterName", "k8sVersion", "topology"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing JSON key %q in output %v", k, got)
		}
	}
}

// TestShowSnapshotNotFound: injected showFn returns ErrSnapshotNotFound → command exits non-zero.
func TestShowSnapshotNotFound(t *testing.T) {
	withShowFn(t, func(_ context.Context, _, _, _ string) (*showResult, error) {
		return nil, snapshot.ErrSnapshotNotFound
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"show", "mycluster", "missing-snap"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for ErrSnapshotNotFound, got nil")
	}
}

// TestShowArgDisambiguation: 1 arg → snap-name (cluster auto-detect); 2 args → cluster+snap.
func TestShowArgDisambiguation(t *testing.T) {
	var capturedCluster, capturedSnap string
	withShowFn(t, func(_ context.Context, _, clusterName, snapName string) (*showResult, error) {
		capturedCluster = clusterName
		capturedSnap = snapName
		return fakeShowResult(), nil
	})

	// 1 arg: snap-name only.
	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"show", "my-snap"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute (1 arg) returned error: %v", err)
	}
	if capturedCluster != "" {
		t.Errorf("1-arg: expected empty clusterName, got %q", capturedCluster)
	}
	if capturedSnap != "my-snap" {
		t.Errorf("1-arg: expected snapName=my-snap, got %q", capturedSnap)
	}

	// 2 args: cluster + snap.
	streams2, _, _ := newTestStreams()
	c2 := NewCommand(log.NoopLogger{}, streams2)
	c2.SetArgs([]string{"show", "mycluster", "my-snap"})
	if err := c2.Execute(); err != nil {
		t.Fatalf("Execute (2 args) returned error: %v", err)
	}
	if capturedCluster != "mycluster" {
		t.Errorf("2-arg: expected clusterName=mycluster, got %q", capturedCluster)
	}
}
