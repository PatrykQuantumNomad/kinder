/*
Copyright 2019 The Kubernetes Authors.

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

package testutil

import (
	"bytes"
	"errors"
	"testing"
)

func TestFakeCmd_WritesOutputToStdout(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := &FakeCmd{Output: []byte("hello")}
	cmd.SetStdout(&buf)
	if err := cmd.Run(); err != nil {
		t.Fatalf("FakeCmd.Run() unexpected error: %v", err)
	}
	if got := buf.String(); got != "hello" {
		t.Errorf("FakeCmd.Run() stdout = %q, want %q", got, "hello")
	}
}

func TestFakeCmd_ReturnsErr(t *testing.T) {
	t.Parallel()
	want := errors.New("fail")
	cmd := &FakeCmd{Err: want}
	if got := cmd.Run(); got != want {
		t.Errorf("FakeCmd.Run() = %v, want %v", got, want)
	}
}

func TestFakeCmd_NoOutputWhenStdoutNotSet(t *testing.T) {
	t.Parallel()
	cmd := &FakeCmd{Output: []byte("should not write")}
	// No SetStdout called — Run should still succeed
	if err := cmd.Run(); err != nil {
		t.Fatalf("FakeCmd.Run() unexpected error: %v", err)
	}
}

func TestFakeNode_ReturnsCmdsInOrder(t *testing.T) {
	t.Parallel()
	cmd0 := &FakeCmd{Err: errors.New("first")}
	cmd1 := &FakeCmd{Err: errors.New("second")}
	node := NewFakeControlPlane("cp1", []*FakeCmd{cmd0, cmd1})

	got0 := node.CommandContext(nil, "kubectl", "apply")
	got1 := node.CommandContext(nil, "kubectl", "wait")

	if got0 != cmd0 {
		t.Errorf("CommandContext call 1: got %v, want cmd0", got0)
	}
	if got1 != cmd1 {
		t.Errorf("CommandContext call 2: got %v, want cmd1", got1)
	}
}

func TestFakeNode_RecordsCalls(t *testing.T) {
	t.Parallel()
	node := NewFakeControlPlane("cp1", nil)
	node.CommandContext(nil, "kubectl", "apply", "-f", "manifest.yaml")
	node.CommandContext(nil, "kubectl", "wait", "--for=condition=Ready")

	if len(node.Calls) != 2 {
		t.Fatalf("expected 2 Calls, got %d", len(node.Calls))
	}
	if node.Calls[0][0] != "kubectl" || node.Calls[0][1] != "apply" {
		t.Errorf("Calls[0] = %v, want [kubectl apply -f manifest.yaml]", node.Calls[0])
	}
	if node.Calls[1][0] != "kubectl" || node.Calls[1][1] != "wait" {
		t.Errorf("Calls[1] = %v, want [kubectl wait --for=condition=Ready]", node.Calls[1])
	}
}

func TestFakeNode_DefaultSuccessCmdWhenQueueExhausted(t *testing.T) {
	t.Parallel()
	node := NewFakeControlPlane("cp1", nil) // empty queue
	cmd := node.CommandContext(nil, "kubectl", "get", "pods")
	if cmd == nil {
		t.Fatal("CommandContext should return a non-nil Cmd when queue is exhausted")
	}
	if err := cmd.Run(); err != nil {
		t.Errorf("default FakeCmd.Run() = %v, want nil", err)
	}
}

func TestFakeNode_StringAndRole(t *testing.T) {
	t.Parallel()
	node := NewFakeControlPlane("my-node", nil)
	if got := node.String(); got != "my-node" {
		t.Errorf("String() = %q, want %q", got, "my-node")
	}
	role, err := node.Role()
	if err != nil {
		t.Fatalf("Role() unexpected error: %v", err)
	}
	if role != "control-plane" {
		t.Errorf("Role() = %q, want %q", role, "control-plane")
	}
}
