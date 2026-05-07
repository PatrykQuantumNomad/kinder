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

package cluster

import (
	"bytes"
	"testing"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

// captured holds the resolved arguments observed by the fake deleteClusterFn.
type captured struct {
	name       string
	kubeconfig string
	called     bool
}

// newTestCommand wires NewCommand with a fake deleteClusterFn so RunE never
// reaches a real *cluster.Provider. Returns the captured-args pointer and a
// run helper that executes the cobra command with the given argv.
//
// Tests must NOT call t.Parallel(): deleteClusterFn is a package-level var
// shared across the whole package; concurrent swaps would race.
func newTestCommand(t *testing.T) (*captured, func(args []string) error) {
	t.Helper()

	cap := &captured{}
	orig := deleteClusterFn
	deleteClusterFn = func(_ log.Logger, name, kubeconfig string) error {
		cap.name = name
		cap.kubeconfig = kubeconfig
		cap.called = true
		return nil
	}
	t.Cleanup(func() { deleteClusterFn = orig })

	streams := cmd.IOStreams{
		In:     bytes.NewReader(nil),
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
	c := NewCommand(log.NoopLogger{}, streams)
	c.SilenceUsage = true
	c.SilenceErrors = true

	run := func(args []string) error {
		c.SetArgs(args)
		return c.Execute()
	}
	return cap, run
}

// TestDeleteCluster_PositionalArg verifies that `kinder delete cluster foo`
// resolves the cluster name from the positional argument. This is the bug fix
// for cobra.NoArgs rejecting all positional input.
func TestDeleteCluster_PositionalArg(t *testing.T) {
	cap, run := newTestCommand(t)
	if err := run([]string{"foo"}); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !cap.called {
		t.Fatal("expected deleteClusterFn to be called")
	}
	if cap.name != "foo" {
		t.Errorf("expected cluster name %q, got %q", "foo", cap.name)
	}
}

// TestDeleteCluster_NameFlag verifies that --name still works when no positional
// arg is provided, preserving backward compatibility.
func TestDeleteCluster_NameFlag(t *testing.T) {
	cap, run := newTestCommand(t)
	if err := run([]string{"--name", "bar"}); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if cap.name != "bar" {
		t.Errorf("expected cluster name %q, got %q", "bar", cap.name)
	}
}

// TestDeleteCluster_PositionalOverridesNameFlag verifies that the positional
// argument takes precedence over --name when both are given.
func TestDeleteCluster_PositionalOverridesNameFlag(t *testing.T) {
	cap, run := newTestCommand(t)
	if err := run([]string{"--name", "from-flag", "from-arg"}); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if cap.name != "from-arg" {
		t.Errorf("expected positional %q to override --name, got %q", "from-arg", cap.name)
	}
}

// TestDeleteCluster_DefaultName verifies that with neither positional arg nor
// --name explicitly set, the cluster.DefaultName ("kind") is used.
func TestDeleteCluster_DefaultName(t *testing.T) {
	cap, run := newTestCommand(t)
	if err := run([]string{}); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if cap.name != "kind" {
		t.Errorf("expected default name %q, got %q", "kind", cap.name)
	}
}

// TestDeleteCluster_RejectsExtraPositionalArgs verifies that passing more than
// one positional argument is rejected by cobra.MaximumNArgs(1).
func TestDeleteCluster_RejectsExtraPositionalArgs(t *testing.T) {
	_, run := newTestCommand(t)
	if err := run([]string{"foo", "bar"}); err == nil {
		t.Fatal("expected error for too many positional args, got nil")
	}
}

// TestDeleteCluster_KubeconfigFlag verifies that --kubeconfig is passed through
// to the delete call.
func TestDeleteCluster_KubeconfigFlag(t *testing.T) {
	cap, run := newTestCommand(t)
	if err := run([]string{"foo", "--kubeconfig", "/tmp/kc"}); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if cap.kubeconfig != "/tmp/kc" {
		t.Errorf("expected kubeconfig %q, got %q", "/tmp/kc", cap.kubeconfig)
	}
}
