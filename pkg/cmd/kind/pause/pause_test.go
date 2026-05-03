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

package pause

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"
)

// withPauseFn swaps the package-level pauseFn for the duration of t and
// restores it on cleanup. Tests use this to avoid spinning a real cluster.
func withPauseFn(t *testing.T, fn func(opts lifecycle.PauseOptions) (*lifecycle.PauseResult, error)) {
	t.Helper()
	prev := pauseFn
	pauseFn = fn
	t.Cleanup(func() { pauseFn = prev })
}

// withResolveClusterName swaps the package-level resolveClusterName helper
// for the duration of t. Tests use this so they don't need a real provider.
func withResolveClusterName(t *testing.T, fn func(args []string) (string, error)) {
	t.Helper()
	prev := resolveClusterName
	resolveClusterName = fn
	t.Cleanup(func() { resolveClusterName = prev })
}

// newStreams returns IOStreams with bytes.Buffer backing each stream so tests
// can inspect output.
func newStreams() (cmd.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return cmd.IOStreams{In: nil, Out: out, ErrOut: errOut}, out, errOut
}

// TestPauseCmd_JSONOutput: --json true, fake lifecycle.Pause returns
// PauseResult{2 nodes both success} → stdout is valid JSON matching
// pauseResult schema; stderr empty; exit 0.
func TestPauseCmd_JSONOutput(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withPauseFn(t, func(opts lifecycle.PauseOptions) (*lifecycle.PauseResult, error) {
		if opts.ClusterName != "kind" {
			t.Errorf("unexpected ClusterName %q", opts.ClusterName)
		}
		return &lifecycle.PauseResult{
			Cluster:  "kind",
			State:    "paused",
			Duration: 1.5,
			Nodes: []lifecycle.NodeResult{
				{Name: "kind-control-plane", Role: "control-plane", Success: true, Duration: 0.7},
				{Name: "kind-worker", Role: "worker", Success: true, Duration: 0.5},
			},
		}, nil
	})

	streams, stdout, stderr := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--json"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}

	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	for _, k := range []string{"cluster", "state", "nodes", "durationSeconds"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing top-level key %q (got %v)", k, mapKeys(got))
		}
	}
	rawNodes, ok := got["nodes"].([]interface{})
	if !ok {
		t.Fatalf("nodes not array: %T", got["nodes"])
	}
	if len(rawNodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(rawNodes))
	}
}

// TestPauseCmd_TextOutput: --json false → stdout has "Cluster paused. Total
// time: " line and per-node successful nodes (per-node text comes from
// lifecycle.Pause via the logger; here we just confirm the final-line summary
// rendered by the command).
func TestPauseCmd_TextOutput(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withPauseFn(t, func(_ lifecycle.PauseOptions) (*lifecycle.PauseResult, error) {
		return &lifecycle.PauseResult{
			Cluster:  "kind",
			State:    "paused",
			Duration: 2.3,
			Nodes: []lifecycle.NodeResult{
				{Name: "kind-control-plane", Role: "control-plane", Success: true, Duration: 0.7},
			},
		}, nil
	})

	streams, stdout, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{}) // no --json flag
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Cluster paused.") {
		t.Errorf("expected 'Cluster paused.' summary line; got: %s", out)
	}
	if !strings.Contains(out, "Total time:") {
		t.Errorf("expected 'Total time:' summary line; got: %s", out)
	}
}

// TestPauseCmd_AlreadyPausedExit0: lifecycle.Pause returns AlreadyPaused=true
// → exit 0 with warning message.
func TestPauseCmd_AlreadyPausedExit0(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withPauseFn(t, func(_ lifecycle.PauseOptions) (*lifecycle.PauseResult, error) {
		return &lifecycle.PauseResult{
			Cluster:       "kind",
			State:         "paused",
			AlreadyPaused: true,
			Nodes:         []lifecycle.NodeResult{},
		}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{})
	if err := c.Execute(); err != nil {
		t.Fatalf("expected exit 0 on AlreadyPaused; got error: %v", err)
	}
}

// TestPauseCmd_NoArgsAutoDetect: args=[], single cluster auto-resolved →
// lifecycle.Pause called with that cluster name.
func TestPauseCmd_NoArgsAutoDetect(t *testing.T) {
	withResolveClusterName(t, func(args []string) (string, error) {
		if len(args) != 0 {
			t.Errorf("expected empty args; got %v", args)
		}
		return "auto-cluster", nil
	})
	called := false
	withPauseFn(t, func(opts lifecycle.PauseOptions) (*lifecycle.PauseResult, error) {
		called = true
		if opts.ClusterName != "auto-cluster" {
			t.Errorf("expected ClusterName=auto-cluster, got %q", opts.ClusterName)
		}
		return &lifecycle.PauseResult{Cluster: "auto-cluster", State: "paused", Nodes: []lifecycle.NodeResult{}}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !called {
		t.Errorf("pauseFn was not called")
	}
}

// TestPauseCmd_PartialFailureExitNonZero: lifecycle.Pause returns aggregated
// error → command exits non-zero; JSON output (if --json) still emitted with
// success=false entries.
func TestPauseCmd_PartialFailureExitNonZero(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withPauseFn(t, func(_ lifecycle.PauseOptions) (*lifecycle.PauseResult, error) {
		return &lifecycle.PauseResult{
			Cluster:  "kind",
			State:    "paused",
			Duration: 1.0,
			Nodes: []lifecycle.NodeResult{
				{Name: "n1", Role: "worker", Success: true, Duration: 0.4},
				{Name: "n2", Role: "worker", Success: false, Error: "stop failed", Duration: 0.6},
			},
		}, fmt.Errorf("aggregated: 1 of 2 stops failed")
	})

	streams, stdout, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--json"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected non-nil error from Execute on partial failure")
	}

	// JSON should still be emitted with both nodes including success=false for n2.
	var got map[string]interface{}
	if jErr := json.Unmarshal(stdout.Bytes(), &got); jErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jErr, stdout.String())
	}
	rawNodes, ok := got["nodes"].([]interface{})
	if !ok || len(rawNodes) != 2 {
		t.Fatalf("expected 2 nodes in JSON, got %v", rawNodes)
	}
	n2, ok := rawNodes[1].(map[string]interface{})
	if !ok {
		t.Fatalf("nodes[1] is not object")
	}
	if success, _ := n2["success"].(bool); success {
		t.Errorf("expected nodes[1].success=false, got true")
	}
}

func mapKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
