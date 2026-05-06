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
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cmd"
	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"
)

// withPruneStoreFn swaps the package-level pruneStoreFn for the duration of t.
func withPruneStoreFn(t *testing.T, fn func(root, clusterName string) (*pruneStoreFns, error)) {
	t.Helper()
	prev := pruneStoreFn
	pruneStoreFn = fn
	t.Cleanup(func() { pruneStoreFn = prev })
}

// fakePruneInfos returns test snapshot.Info values for prune tests.
func fakePruneInfos(n int) []snapshot.Info {
	now := time.Now()
	infos := make([]snapshot.Info, n)
	for i := 0; i < n; i++ {
		infos[i] = snapshot.Info{
			Name:      fmt.Sprintf("snap-%02d", i),
			ClusterName: "mycluster",
			Size:      int64((i + 1)) * 100 * 1024 * 1024,
			CreatedAt: now.Add(-time.Duration(i+1) * 24 * time.Hour),
			Status:    "ok",
		}
	}
	return infos
}

// newPruneStreams returns IOStreams with a custom stdin for prune tests.
func newPruneStreams(stdin string) cmd.IOStreams {
	streams, _, _ := newTestStreams()
	streams.In = strings.NewReader(stdin)
	return streams
}

// TestPruneNoFlagsRefused: no policy flags → error message lists all 3 flags.
func TestPruneNoFlagsRefused(t *testing.T) {
	withPruneStoreFn(t, func(_, _ string) (*pruneStoreFns, error) {
		return &pruneStoreFns{
			list:   func(_ context.Context) ([]snapshot.Info, error) { return nil, nil },
			delete: func(_ context.Context, _ string) error { return nil },
		}, nil
	})

	c := NewCommand(log.NoopLogger{}, newPruneStreams(""))
	c.SetArgs([]string{"prune", "mycluster"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error with no policy flags, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "--keep-last") {
		t.Errorf("expected --keep-last in error message; got: %s", errStr)
	}
	if !strings.Contains(errStr, "--older-than") {
		t.Errorf("expected --older-than in error message; got: %s", errStr)
	}
	if !strings.Contains(errStr, "--max-size") {
		t.Errorf("expected --max-size in error message; got: %s", errStr)
	}
}

// TestPruneNoVictims: store has snapshots but policy matches none → "Nothing to delete."
func TestPruneNoVictims(t *testing.T) {
	// Keep 10, but only have 2 snapshots → nothing deleted.
	var deleteCalled bool
	withPruneStoreFn(t, func(_, _ string) (*pruneStoreFns, error) {
		return &pruneStoreFns{
			list:   func(_ context.Context) ([]snapshot.Info, error) { return fakePruneInfos(2), nil },
			delete: func(_ context.Context, _ string) error { deleteCalled = true; return nil },
		}, nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"prune", "mycluster", "--keep-last=10"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Nothing to delete") {
		t.Errorf("expected 'Nothing to delete' output; got:\n%s", stdout.String())
	}
	if deleteCalled {
		t.Errorf("delete must not be called when no victims")
	}
}

// TestPrunePromptYes: 2 victims, stdin="y\n" → both Delete calls invoked.
func TestPrunePromptYes(t *testing.T) {
	var deleted []string
	withPruneStoreFn(t, func(_, _ string) (*pruneStoreFns, error) {
		return &pruneStoreFns{
			list:   func(_ context.Context) ([]snapshot.Info, error) { return fakePruneInfos(4), nil },
			delete: func(_ context.Context, name string) error { deleted = append(deleted, name); return nil },
		}, nil
	})

	// --keep-last=2 with 4 snapshots → 2 victims.
	streams := newPruneStreams("y\n")
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"prune", "mycluster", "--keep-last=2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("expected 2 delete calls, got %d: %v", len(deleted), deleted)
	}
}

// TestPrunePromptNo: 2 victims, stdin="n\n" → returns "aborted" error; NO Delete calls.
func TestPrunePromptNo(t *testing.T) {
	var deleteCalled bool
	withPruneStoreFn(t, func(_, _ string) (*pruneStoreFns, error) {
		return &pruneStoreFns{
			list:   func(_ context.Context) ([]snapshot.Info, error) { return fakePruneInfos(4), nil },
			delete: func(_ context.Context, _ string) error { deleteCalled = true; return nil },
		}, nil
	})

	streams := newPruneStreams("n\n")
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"prune", "mycluster", "--keep-last=2"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error (aborted) from 'n' response, got nil")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected 'aborted' in error; got: %v", err)
	}
	if deleteCalled {
		t.Errorf("delete must not be called when user answers 'n'")
	}
}

// TestPruneYesFlagSkipsPrompt: --yes set → no prompt read; Delete called.
func TestPruneYesFlagSkipsPrompt(t *testing.T) {
	var deleted []string
	withPruneStoreFn(t, func(_, _ string) (*pruneStoreFns, error) {
		return &pruneStoreFns{
			list:   func(_ context.Context) ([]snapshot.Info, error) { return fakePruneInfos(4), nil },
			delete: func(_ context.Context, name string) error { deleted = append(deleted, name); return nil },
		}, nil
	})

	// stdin has no content — if prompt is attempted, ReadString blocks/fails.
	// Use empty stdin; test verifies deletion happened without waiting for input.
	streams := newPruneStreams("")
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"prune", "mycluster", "--keep-last=2", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(deleted) == 0 {
		t.Errorf("expected delete calls with --yes; got none")
	}
}

// TestPrunePropagatesDeleteError: injected Delete returns error → aggregated err mentions snapshot name.
func TestPrunePropagatesDeleteError(t *testing.T) {
	withPruneStoreFn(t, func(_, _ string) (*pruneStoreFns, error) {
		return &pruneStoreFns{
			list: func(_ context.Context) ([]snapshot.Info, error) { return fakePruneInfos(3), nil },
			delete: func(_ context.Context, name string) error {
				return fmt.Errorf("disk failure for %s", name)
			},
		}, nil
	})

	streams := newPruneStreams("")
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"prune", "mycluster", "--keep-last=1", "--yes"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected aggregate error from delete failures, got nil")
	}
	// The error must mention at least one snapshot name.
	if !strings.Contains(err.Error(), "snap-") {
		t.Errorf("expected snapshot name in aggregate error; got: %v", err)
	}
}

// TestParseSize: table-driven tests for parseSize.
func TestParseSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"5G", 5 * 1024 * 1024 * 1024, false},
		{"5GiB", 5 * 1024 * 1024 * 1024, false},
		{"500M", 500 * 1024 * 1024, false},
		{"500MiB", 500 * 1024 * 1024, false},
		{"1024K", 1024 * 1024, false},
		{"1024KiB", 1024 * 1024, false},
		{"2T", 2 * 1024 * 1024 * 1024 * 1024, false},
		{"1234", 1234, false},
		{"", 0, false},
		{"garbage", 0, true},
		{"-5G", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseSize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseSize(%q) expected error, got nil (result=%d)", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseSize(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
