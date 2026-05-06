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
	"bytes"
	"encoding/json"
	"testing"

	"sigs.k8s.io/kind/pkg/cmd"
	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"
)

// withCreateFn swaps the package-level createFn for the duration of t and
// restores it on cleanup. Tests use this to avoid spinning a real cluster.
func withCreateFn(t *testing.T, fn func(opts snapshot.CreateOptions) (*snapshot.CreateResult, error)) {
	t.Helper()
	prev := createFn
	createFn = fn
	t.Cleanup(func() { createFn = prev })
}

// newTestStreams returns IOStreams backed by bytes.Buffer for test inspection.
func newTestStreams() (cmd.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return cmd.IOStreams{In: nil, Out: out, ErrOut: errOut}, out, errOut
}

// fakeCreateResult returns a non-nil CreateResult for use in tests.
func fakeCreateResult(snapName string) *snapshot.CreateResult {
	return &snapshot.CreateResult{
		SnapName:  snapName,
		Path:      "/tmp/.kinder/snapshots/mycluster/" + snapName + ".tar.gz",
		Size:      1024 * 1024 * 512, // 512 MiB
		DurationS: 1.5,
	}
}

// TestCreateCommand_Defaults: 1 positional arg (cluster name), snap-name
// not provided → createFn called with ClusterName=mycluster, SnapName="".
func TestCreateCommand_Defaults(t *testing.T) {
	var captured snapshot.CreateOptions
	withCreateFn(t, func(opts snapshot.CreateOptions) (*snapshot.CreateResult, error) {
		captured = opts
		return fakeCreateResult("snap-20260101-120000"), nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"create", "mycluster"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if captured.ClusterName != "mycluster" {
		t.Errorf("expected ClusterName=mycluster, got %q", captured.ClusterName)
	}
	if captured.SnapName != "" {
		t.Errorf("expected SnapName empty (auto-generate), got %q", captured.SnapName)
	}
}

// TestCreateCommand_TwoPositionals: 2 positional args → ClusterName and SnapName both set.
func TestCreateCommand_TwoPositionals(t *testing.T) {
	var captured snapshot.CreateOptions
	withCreateFn(t, func(opts snapshot.CreateOptions) (*snapshot.CreateResult, error) {
		captured = opts
		return fakeCreateResult("my-snap"), nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"create", "mycluster", "my-snap"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if captured.ClusterName != "mycluster" {
		t.Errorf("expected ClusterName=mycluster, got %q", captured.ClusterName)
	}
	if captured.SnapName != "my-snap" {
		t.Errorf("expected SnapName=my-snap, got %q", captured.SnapName)
	}
}

// TestCreateCommand_NameFlag: --name flag overrides positional snap-name.
func TestCreateCommand_NameFlag(t *testing.T) {
	var captured snapshot.CreateOptions
	withCreateFn(t, func(opts snapshot.CreateOptions) (*snapshot.CreateResult, error) {
		captured = opts
		return fakeCreateResult(opts.SnapName), nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"create", "--name=via-flag"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if captured.SnapName != "via-flag" {
		t.Errorf("expected SnapName=via-flag, got %q", captured.SnapName)
	}
}

// TestCreateCommand_JSONOutput: --json flag → stdout is valid JSON containing
// CreateResult fields (snapName, path, size).
func TestCreateCommand_JSONOutput(t *testing.T) {
	withCreateFn(t, func(opts snapshot.CreateOptions) (*snapshot.CreateResult, error) {
		return fakeCreateResult("snap-json-test"), nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"create", "mycluster", "--json"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	// CreateResult uses default Go JSON encoding (capitalized field names, no json tags).
	for _, k := range []string{"SnapName", "Path", "Size"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing JSON key %q in output %v", k, got)
		}
	}
}

// TestCreateCommand_TextOutput: no --json → text output contains snapshot name and path.
func TestCreateCommand_TextOutput(t *testing.T) {
	withCreateFn(t, func(opts snapshot.CreateOptions) (*snapshot.CreateResult, error) {
		return fakeCreateResult("snap-text-test"), nil
	})

	streams, stdout, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"create", "mycluster"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("snap-text-test")) {
		t.Errorf("expected snap name in output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("Path:")) {
		t.Errorf("expected Path: line in output, got: %s", out)
	}
}

// TestCreateCommand_PropagatesError: createFn returns error → command exits non-zero.
func TestCreateCommand_PropagatesError(t *testing.T) {
	withCreateFn(t, func(opts snapshot.CreateOptions) (*snapshot.CreateResult, error) {
		return nil, snapshot.ErrSnapshotNotFound
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"create", "mycluster"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected non-nil error from Execute, got nil")
	}
}
