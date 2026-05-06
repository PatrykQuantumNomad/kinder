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
	sigsyaml "sigs.k8s.io/yaml"
)

// withListFn swaps the package-level listFn for the duration of t.
func withListFn(t *testing.T, fn func(ctx context.Context, root, clusterName string) ([]snapshot.Info, error)) {
	t.Helper()
	prev := listFn
	listFn = fn
	t.Cleanup(func() { listFn = prev })
}

// fakeInfos returns sample snapshot.Info values for testing.
func fakeInfos() []snapshot.Info {
	now := time.Now()
	return []snapshot.Info{
		{
			Name:      "snap-20260101-120000",
			ClusterName: "mycluster",
			Path:      "/tmp/.kinder/snapshots/mycluster/snap-20260101-120000.tar.gz",
			Size:      512 * 1024 * 1024,
			CreatedAt: now.Add(-2 * time.Hour),
			Status:    "ok",
			Metadata: &snapshot.Metadata{
				K8sVersion:    "v1.31.2",
				AddonVersions: map[string]string{"calico": "v3.28.0", "metrics-server": "v0.7.1"},
			},
		},
		{
			Name:      "snap-20260102-090000",
			ClusterName: "mycluster",
			Path:      "/tmp/.kinder/snapshots/mycluster/snap-20260102-090000.tar.gz",
			Size:      1024 * 1024 * 1024,
			CreatedAt: now.Add(-26 * time.Hour),
			Status:    "ok",
			Metadata: &snapshot.Metadata{
				K8sVersion:    "v1.31.2",
				AddonVersions: map[string]string{"calico": "v3.28.0"},
			},
		},
	}
}

// TestListTableOutput: injected listFn returns 2 infos; output contains the
// exact column header and one row per snapshot.
func TestListTableOutput(t *testing.T) {
	withListFn(t, func(_ context.Context, _, _ string) ([]snapshot.Info, error) {
		return fakeInfos(), nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"list", "mycluster"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	out := stdout.String()
	// Header must be present.
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "AGE") ||
		!strings.Contains(out, "SIZE") || !strings.Contains(out, "K8S") ||
		!strings.Contains(out, "ADDONS") || !strings.Contains(out, "STATUS") {
		t.Errorf("expected column headers NAME/AGE/SIZE/K8S/ADDONS/STATUS; got:\n%s", out)
	}
	// Both snapshot names must appear.
	if !strings.Contains(out, "snap-20260101-120000") {
		t.Errorf("expected snap-20260101-120000 in output; got:\n%s", out)
	}
	if !strings.Contains(out, "snap-20260102-090000") {
		t.Errorf("expected snap-20260102-090000 in output; got:\n%s", out)
	}
}

// TestListJSONOutput: --output=json → valid JSON array with correct length.
func TestListJSONOutput(t *testing.T) {
	withListFn(t, func(_ context.Context, _, _ string) ([]snapshot.Info, error) {
		return fakeInfos(), nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"list", "mycluster", "--output=json"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var got []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON array: %v\n%s", err, stdout.String())
	}
	if len(got) != 2 {
		t.Errorf("expected 2 items in JSON array, got %d", len(got))
	}
}

// TestListYAMLOutput: --output=yaml → valid YAML that parses back as a slice.
func TestListYAMLOutput(t *testing.T) {
	withListFn(t, func(_ context.Context, _, _ string) ([]snapshot.Info, error) {
		return fakeInfos(), nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"list", "mycluster", "--output=yaml"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var got []map[string]interface{}
	if err := sigsyaml.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid YAML: %v\n%s", err, stdout.String())
	}
	if len(got) != 2 {
		t.Errorf("expected 2 items in YAML, got %d", len(got))
	}
}

// TestListEmpty: listFn returns empty slice → only header line printed.
func TestListEmpty(t *testing.T) {
	withListFn(t, func(_ context.Context, _, _ string) ([]snapshot.Info, error) {
		return []snapshot.Info{}, nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"list", "mycluster"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	out := stdout.String()
	// Header must still be present.
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected header line even with empty list; got:\n%s", out)
	}
	// No snapshot-looking lines.
	if strings.Contains(out, "snap-") {
		t.Errorf("unexpected snapshot rows in empty list output:\n%s", out)
	}
}

// TestListNoTrunc: default invocation truncates long ADDONS; --no-trunc prints full.
func TestListNoTrunc(t *testing.T) {
	longAddons := map[string]string{}
	for i := 0; i < 15; i++ {
		key := strings.Repeat("a", 4) + strings.Repeat("b", i)
		longAddons[key] = "v1.0." + string(rune('0'+i))
	}
	withListFn(t, func(_ context.Context, _, _ string) ([]snapshot.Info, error) {
		return []snapshot.Info{
			{
				Name:      "snap-long-addons",
				ClusterName: "mycluster",
				Size:      100,
				CreatedAt: time.Now().Add(-time.Hour),
				Status:    "ok",
				Metadata: &snapshot.Metadata{
					K8sVersion:    "v1.31.2",
					AddonVersions: longAddons,
				},
			},
		}, nil
	})

	// Default: truncated.
	streams1, stdout1, _ := newTestStreams()
	c1 := NewCommand(log.NoopLogger{}, streams1)
	c1.SetArgs([]string{"list", "mycluster"})
	if err := c1.Execute(); err != nil {
		t.Fatalf("Execute (default) returned error: %v", err)
	}
	out1 := stdout1.String()

	// --no-trunc: full.
	streams2, stdout2, _ := newTestStreams()
	c2 := NewCommand(log.NoopLogger{}, streams2)
	c2.SetArgs([]string{"list", "mycluster", "--no-trunc"})
	if err := c2.Execute(); err != nil {
		t.Fatalf("Execute (--no-trunc) returned error: %v", err)
	}
	out2 := stdout2.String()

	// Default output must contain the ellipsis truncation indicator.
	if !strings.Contains(out1, "…") {
		t.Errorf("expected truncation ellipsis in default output; got:\n%s", out1)
	}
	// --no-trunc output must NOT contain the ellipsis.
	if strings.Contains(out2, "…") {
		t.Errorf("expected no ellipsis with --no-trunc; got:\n%s", out2)
	}
	// --no-trunc output must be longer (more addon info).
	if len(out2) <= len(out1) {
		t.Errorf("expected --no-trunc output longer than default; default=%d, no-trunc=%d", len(out1), len(out2))
	}
}
