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

package lifecycle

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

// fakeNode is a minimal nodes.Node implementation for testing.
type fakeNode struct {
	name string
	role string
	err  error
}

var _ nodes.Node = (*fakeNode)(nil)

func (f *fakeNode) String() string                      { return f.name }
func (f *fakeNode) Role() (string, error)               { return f.role, f.err }
func (f *fakeNode) IP() (string, string, error)         { return "", "", nil }
func (f *fakeNode) SerialLogs(_ io.Writer) error        { return nil }
func (f *fakeNode) Command(c string, a ...string) exec.Cmd {
	return defaultCmder(c, a...)
}
func (f *fakeNode) CommandContext(_ context.Context, c string, a ...string) exec.Cmd {
	return defaultCmder(c, a...)
}

// fakeCmd implements exec.Cmd, returning a canned stdout line and optional err.
type fakeCmd struct {
	stdout string
	err    error
	stdoutW io.Writer
}

var _ exec.Cmd = (*fakeCmd)(nil)

func (f *fakeCmd) Run() error {
	if f.stdoutW != nil && f.stdout != "" {
		_, _ = f.stdoutW.Write([]byte(f.stdout))
	}
	return f.err
}
func (f *fakeCmd) SetEnv(_ ...string) exec.Cmd        { return f }
func (f *fakeCmd) SetStdin(_ io.Reader) exec.Cmd      { return f }
func (f *fakeCmd) SetStdout(w io.Writer) exec.Cmd     { f.stdoutW = w; return f }
func (f *fakeCmd) SetStderr(_ io.Writer) exec.Cmd     { return f }

// withCmder swaps the package-level cmder for the duration of the test.
func withCmder(t *testing.T, c Cmder) {
	t.Helper()
	prev := defaultCmder
	defaultCmder = c
	t.Cleanup(func() { defaultCmder = prev })
}

// fakeCmderByName returns canned fakeCmds keyed by container name (the last arg).
func fakeCmderByName(byName map[string]*fakeCmd) Cmder {
	return func(_ string, args ...string) exec.Cmd {
		// container name is the last arg in `inspect --format ... <name>`
		name := ""
		if len(args) > 0 {
			name = args[len(args)-1]
		}
		if c, ok := byName[name]; ok {
			// return a fresh copy so SetStdout works per-call
			return &fakeCmd{stdout: c.stdout, err: c.err}
		}
		return &fakeCmd{err: fmt.Errorf("no fake for %q", name)}
	}
}

// fakeLister is a stand-in for the cluster lister contract used by ResolveClusterName.
type fakeLister struct {
	names []string
	err   error
}

func (f *fakeLister) List() ([]string, error) { return f.names, f.err }

// --- ContainerState tests ---

func TestContainerState_Running(t *testing.T) {
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"node-a": {stdout: "running\n"},
	}))
	got, err := ContainerState("docker", "node-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "running" {
		t.Errorf("got %q, want %q", got, "running")
	}
}

func TestContainerState_Exited(t *testing.T) {
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"node-a": {stdout: "exited\n"},
	}))
	got, err := ContainerState("docker", "node-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "exited" {
		t.Errorf("got %q, want %q", got, "exited")
	}
}

func TestContainerState_Error(t *testing.T) {
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"node-a": {err: fmt.Errorf("inspect failed")},
	}))
	got, err := ContainerState("docker", "node-a")
	if err == nil {
		t.Fatalf("expected error, got nil (state=%q)", got)
	}
	if got != "" {
		t.Errorf("on error want empty state, got %q", got)
	}
}

// --- ClusterStatus tests ---

func TestClusterStatus_AllRunning(t *testing.T) {
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"n1": {stdout: "running\n"},
		"n2": {stdout: "running\n"},
		"n3": {stdout: "running\n"},
	}))
	ns := []nodes.Node{
		&fakeNode{name: "n1"},
		&fakeNode{name: "n2"},
		&fakeNode{name: "n3"},
	}
	got := ClusterStatus("docker", ns)
	if got != "Running" {
		t.Errorf("got %q, want %q", got, "Running")
	}
}

func TestClusterStatus_AllStopped(t *testing.T) {
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"n1": {stdout: "exited\n"},
		"n2": {stdout: "exited\n"},
		"n3": {stdout: "exited\n"},
	}))
	ns := []nodes.Node{
		&fakeNode{name: "n1"},
		&fakeNode{name: "n2"},
		&fakeNode{name: "n3"},
	}
	got := ClusterStatus("docker", ns)
	if got != "Paused" {
		t.Errorf("got %q, want %q", got, "Paused")
	}
}

func TestClusterStatus_Mixed(t *testing.T) {
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"n1": {stdout: "running\n"},
		"n2": {stdout: "running\n"},
		"n3": {stdout: "exited\n"},
	}))
	ns := []nodes.Node{
		&fakeNode{name: "n1"},
		&fakeNode{name: "n2"},
		&fakeNode{name: "n3"},
	}
	got := ClusterStatus("docker", ns)
	if got != "Error" {
		t.Errorf("got %q, want %q", got, "Error")
	}
}

func TestClusterStatus_Empty(t *testing.T) {
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{}))
	got := ClusterStatus("docker", nil)
	if got != "Error" {
		t.Errorf("got %q, want %q", got, "Error")
	}
}

// --- ResolveClusterName tests ---

func TestResolveClusterName_OneArg(t *testing.T) {
	lister := &fakeLister{names: []string{"x", "y"}}
	got, err := ResolveClusterName([]string{"foo"}, lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "foo" {
		t.Errorf("got %q, want %q", got, "foo")
	}
}

func TestResolveClusterName_AutoSingle(t *testing.T) {
	lister := &fakeLister{names: []string{"only"}}
	got, err := ResolveClusterName([]string{}, lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "only" {
		t.Errorf("got %q, want %q", got, "only")
	}
}

func TestResolveClusterName_AutoNone(t *testing.T) {
	lister := &fakeLister{names: []string{}}
	_, err := ResolveClusterName([]string{}, lister)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no kind clusters found") {
		t.Errorf("error %q does not contain %q", err.Error(), "no kind clusters found")
	}
}

func TestResolveClusterName_AutoMulti(t *testing.T) {
	lister := &fakeLister{names: []string{"a", "b"}}
	_, err := ResolveClusterName([]string{}, lister)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "a") || !strings.Contains(msg, "b") {
		t.Errorf("error %q should list both cluster names a, b", msg)
	}
}

// --- ClassifyNodes tests ---

func TestClassifyNodes_HA(t *testing.T) {
	allNodes := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
		&fakeNode{name: "w2", role: "worker"},
		&fakeNode{name: "lb", role: "external-load-balancer"},
	}
	cp, workers, lb, err := ClassifyNodes(allNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cp) != 3 {
		t.Errorf("cp count = %d, want 3", len(cp))
	}
	if len(workers) != 2 {
		t.Errorf("workers count = %d, want 2", len(workers))
	}
	if lb == nil || lb.String() != "lb" {
		t.Errorf("lb = %v, want node named lb", lb)
	}
}

func TestClassifyNodes_SingleNode(t *testing.T) {
	allNodes := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
	}
	cp, workers, lb, err := ClassifyNodes(allNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cp) != 1 {
		t.Errorf("cp count = %d, want 1", len(cp))
	}
	if len(workers) != 0 {
		t.Errorf("workers count = %d, want 0", len(workers))
	}
	if lb != nil {
		t.Errorf("lb = %v, want nil", lb)
	}
}
