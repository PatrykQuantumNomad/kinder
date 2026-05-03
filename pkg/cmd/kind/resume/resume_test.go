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

package resume

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"
)

// withResumeFn swaps the package-level resumeFn for the duration of t and
// restores it on cleanup. Tests use this to avoid spinning a real cluster.
func withResumeFn(t *testing.T, fn func(opts lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error)) {
	t.Helper()
	prev := resumeFn
	resumeFn = fn
	t.Cleanup(func() { resumeFn = prev })
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

// TestResumeCmd_JSONOutput: --json true, fake lifecycle.Resume returns
// ResumeResult{2 nodes both success, readiness 3.5s} → stdout valid JSON
// matching ResumeResult schema; stderr empty; exit 0.
func TestResumeCmd_JSONOutput(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withResumeFn(t, func(opts lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		if opts.ClusterName != "kind" {
			t.Errorf("unexpected ClusterName %q", opts.ClusterName)
		}
		return &lifecycle.ResumeResult{
			Cluster:          "kind",
			State:            "resumed",
			Duration:         5.0,
			ReadinessSeconds: 3.5,
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
	for _, k := range []string{"cluster", "state", "nodes", "readinessSeconds", "durationSeconds"} {
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
	if rs, _ := got["readinessSeconds"].(float64); rs != 3.5 {
		t.Errorf("expected readinessSeconds=3.5, got %v", got["readinessSeconds"])
	}
}

// TestResumeCmd_TextOutput: --json false → stdout has "Cluster resumed. Total
// time: " line. (Per-node text comes from lifecycle.Resume via the logger; the
// command emits only the final summary line to avoid duplication.)
func TestResumeCmd_TextOutput(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withResumeFn(t, func(_ lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		return &lifecycle.ResumeResult{
			Cluster:  "kind",
			State:    "resumed",
			Duration: 4.2,
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
	if !strings.Contains(out, "Cluster resumed.") {
		t.Errorf("expected 'Cluster resumed.' summary line; got: %s", out)
	}
	if !strings.Contains(out, "Total time:") {
		t.Errorf("expected 'Total time:' summary line; got: %s", out)
	}
}

// TestResumeCmd_AlreadyRunningExit0: lifecycle.Resume returns
// AlreadyRunning=true → exit 0, no error.
func TestResumeCmd_AlreadyRunningExit0(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withResumeFn(t, func(_ lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		return &lifecycle.ResumeResult{
			Cluster:        "kind",
			State:          "resumed",
			AlreadyRunning: true,
			Nodes:          []lifecycle.NodeResult{},
		}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{})
	if err := c.Execute(); err != nil {
		t.Fatalf("expected exit 0 on AlreadyRunning; got error: %v", err)
	}
}

// TestResumeCmd_NoArgsAutoDetect: args=[], single cluster auto-resolved →
// lifecycle.Resume called with that cluster name.
func TestResumeCmd_NoArgsAutoDetect(t *testing.T) {
	withResolveClusterName(t, func(args []string) (string, error) {
		if len(args) != 0 {
			t.Errorf("expected empty args; got %v", args)
		}
		return "auto-cluster", nil
	})
	called := false
	withResumeFn(t, func(opts lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		called = true
		if opts.ClusterName != "auto-cluster" {
			t.Errorf("expected ClusterName=auto-cluster, got %q", opts.ClusterName)
		}
		return &lifecycle.ResumeResult{Cluster: "auto-cluster", State: "resumed", Nodes: []lifecycle.NodeResult{}}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !called {
		t.Errorf("resumeFn was not called")
	}
}

// TestResumeCmd_ReadinessTimeoutExitNonZero: lifecycle.Resume returns timeout
// error → command exits non-zero; if --json, JSON still emitted with what was
// captured.
func TestResumeCmd_ReadinessTimeoutExitNonZero(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	withResumeFn(t, func(_ lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		return &lifecycle.ResumeResult{
			Cluster:          "kind",
			State:            "resumed",
			Duration:         301.0,
			ReadinessSeconds: 300.0,
			Nodes: []lifecycle.NodeResult{
				{Name: "n1", Role: "control-plane", Success: true, Duration: 0.5},
			},
		}, fmt.Errorf("timed out waiting for nodes Ready: deadline exceeded")
	})

	streams, stdout, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--json"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected non-nil error from Execute on readiness timeout")
	}

	// JSON should still be emitted with what was captured.
	var got map[string]interface{}
	if jErr := json.Unmarshal(stdout.Bytes(), &got); jErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jErr, stdout.String())
	}
	if state, _ := got["state"].(string); state != "resumed" {
		t.Errorf("expected state=resumed in JSON, got %v", got["state"])
	}
}

// TestResumeCmd_WaitFlagPropagated: --wait=600 → fake resumeFn captures
// WaitTimeout = 600 * time.Second.
func TestResumeCmd_WaitFlagPropagated(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	var captured time.Duration
	withResumeFn(t, func(opts lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		captured = opts.WaitTimeout
		return &lifecycle.ResumeResult{Cluster: "kind", State: "resumed", Nodes: []lifecycle.NodeResult{}}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--wait=600"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	want := 600 * time.Second
	if captured != want {
		t.Errorf("expected WaitTimeout=%v, got %v", want, captured)
	}
}

// TestResumeCmd_TimeoutFlagPropagated: --timeout=45 → fake resumeFn captures
// StartTimeout = 45 * time.Second.
func TestResumeCmd_TimeoutFlagPropagated(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	var captured time.Duration
	withResumeFn(t, func(opts lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		captured = opts.StartTimeout
		return &lifecycle.ResumeResult{Cluster: "kind", State: "resumed", Nodes: []lifecycle.NodeResult{}}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--timeout=45"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	want := 45 * time.Second
	if captured != want {
		t.Errorf("expected StartTimeout=%v, got %v", want, captured)
	}
}

// TestResumeCmd_NegativeWaitRejected: --wait=-1 → command exits non-zero with
// a clear error message.
func TestResumeCmd_NegativeWaitRejected(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	called := false
	withResumeFn(t, func(_ lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		called = true
		return &lifecycle.ResumeResult{Cluster: "kind", State: "resumed", Nodes: []lifecycle.NodeResult{}}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--wait=-1"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for negative --wait, got nil")
	}
	if called {
		t.Errorf("resumeFn must not be called when --wait validation fails")
	}
}

// TestResumeCmd_NegativeTimeoutRejected: --timeout=-1 → command exits non-zero.
func TestResumeCmd_NegativeTimeoutRejected(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) { return "kind", nil })
	called := false
	withResumeFn(t, func(_ lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		called = true
		return &lifecycle.ResumeResult{Cluster: "kind", State: "resumed", Nodes: []lifecycle.NodeResult{}}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--timeout=-1"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for negative --timeout, got nil")
	}
	if called {
		t.Errorf("resumeFn must not be called when --timeout validation fails")
	}
}

// TestResumeCmd_ResolveError: resolveClusterName returns error → Execute
// returns that error and resumeFn is not called.
func TestResumeCmd_ResolveError(t *testing.T) {
	withResolveClusterName(t, func(_ []string) (string, error) {
		return "", fmt.Errorf("no kind clusters found")
	})
	called := false
	withResumeFn(t, func(_ lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		called = true
		return &lifecycle.ResumeResult{Cluster: "", State: "resumed", Nodes: []lifecycle.NodeResult{}}, nil
	})

	streams, _, _ := newStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected resolve error, got nil")
	}
	if called {
		t.Errorf("resumeFn must not be called when resolve fails")
	}
}

func mapKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
