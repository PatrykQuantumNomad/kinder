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

package dev

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"sigs.k8s.io/kind/pkg/exec"
)

// --- exec.Cmder fake (mirrors pkg/internal/lifecycle test infra) ---

// recordedExecCall captures a single (name, args) invocation through the fake
// Cmder so tests can assert exact argv shape.
type recordedExecCall struct {
	name string
	args []string
}

// fakeExecCmder is an exec.Cmder that records calls and returns canned
// results. lookup may be nil; if non-nil it returns (stdout, err) for the
// invocation. If lookup is nil, calls succeed with empty stdout.
type fakeExecCmder struct {
	mu     sync.Mutex
	calls  []recordedExecCall
	lookup func(name string, args []string) (stdout string, err error)
}

var _ exec.Cmder = (*fakeExecCmder)(nil)

func (f *fakeExecCmder) Command(name string, arg ...string) exec.Cmd {
	return f.record(name, arg)
}

func (f *fakeExecCmder) CommandContext(_ context.Context, name string, arg ...string) exec.Cmd {
	return f.record(name, arg)
}

func (f *fakeExecCmder) record(name string, arg []string) exec.Cmd {
	f.mu.Lock()
	argsCopy := make([]string, len(arg))
	copy(argsCopy, arg)
	f.calls = append(f.calls, recordedExecCall{name: name, args: argsCopy})
	f.mu.Unlock()
	stdout := ""
	var err error
	if f.lookup != nil {
		stdout, err = f.lookup(name, arg)
	}
	return &fakeExecCmd{stdout: stdout, err: err}
}

func (f *fakeExecCmder) snapshot() []recordedExecCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedExecCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// fakeExecCmd is an exec.Cmd that ignores stdin/stderr wiring and returns
// the configured stdout / err from Run().
type fakeExecCmd struct {
	stdout  string
	err     error
	stdoutW io.Writer
}

var _ exec.Cmd = (*fakeExecCmd)(nil)

func (c *fakeExecCmd) Run() error {
	if c.stdoutW != nil && c.stdout != "" {
		_, _ = c.stdoutW.Write([]byte(c.stdout))
	}
	return c.err
}
func (c *fakeExecCmd) SetEnv(_ ...string) exec.Cmd    { return c }
func (c *fakeExecCmd) SetStdin(_ io.Reader) exec.Cmd  { return c }
func (c *fakeExecCmd) SetStdout(w io.Writer) exec.Cmd { c.stdoutW = w; return c }
func (c *fakeExecCmd) SetStderr(_ io.Writer) exec.Cmd { return c }

// withDevCmder swaps the package-level devCmder used by build/load/rollout
// for the duration of the test.
func withDevCmder(t *testing.T, c exec.Cmder) {
	t.Helper()
	prev := devCmder
	devCmder = c
	t.Cleanup(func() { devCmder = prev })
}

// --- BuildImage tests ---

func TestBuildImage_PassesArgs(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)

	if err := BuildImage(context.Background(), "docker", "myimage:latest", "/tmp/ctx"); err != nil {
		t.Fatalf("BuildImage returned error: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 exec call, got %d: %#v", len(calls), calls)
	}
	c := calls[0]
	if c.name != "docker" {
		t.Errorf("name = %q, want %q", c.name, "docker")
	}
	wantArgs := []string{"build", "-t", "myimage:latest", "/tmp/ctx"}
	if len(c.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", c.args, wantArgs)
	}
	for i := range wantArgs {
		if c.args[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q, want %q (full args: %v)", i, c.args[i], wantArgs[i], c.args)
		}
	}
}

func TestBuildImage_PropagatesError(t *testing.T) {
	rec := &fakeExecCmder{
		lookup: func(_ string, _ []string) (string, error) {
			return "", errors.New("build failed")
		},
	}
	withDevCmder(t, rec)

	err := BuildImage(context.Background(), "docker", "img:tag", "/ctx")
	if err == nil {
		t.Fatal("expected error from BuildImage, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "build failed") {
		t.Errorf("error %q should contain underlying message %q", msg, "build failed")
	}
	if !strings.Contains(msg, "docker build") {
		t.Errorf("error %q should mention 'docker build' for context", msg)
	}
}

func TestBuildImage_RejectsEmptyBinaryName(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)

	err := BuildImage(context.Background(), "", "img:tag", "/ctx")
	if err == nil {
		t.Fatal("expected error for empty binaryName, got nil")
	}
	if !strings.Contains(err.Error(), "binaryName") {
		t.Errorf("error %q should mention binaryName", err.Error())
	}
	if len(rec.snapshot()) != 0 {
		t.Errorf("no exec call should be made on validation failure; got %#v", rec.snapshot())
	}
}

func TestBuildImage_RejectsEmptyImageTag(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)

	err := BuildImage(context.Background(), "docker", "", "/ctx")
	if err == nil {
		t.Fatal("expected error for empty imageTag, got nil")
	}
	if !strings.Contains(err.Error(), "imageTag") {
		t.Errorf("error %q should mention imageTag", err.Error())
	}
	if len(rec.snapshot()) != 0 {
		t.Errorf("no exec call should be made on validation failure; got %#v", rec.snapshot())
	}
}

func TestBuildImage_RejectsEmptyContextDir(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)

	err := BuildImage(context.Background(), "docker", "img:tag", "")
	if err == nil {
		t.Fatal("expected error for empty contextDir, got nil")
	}
	if !strings.Contains(err.Error(), "contextDir") {
		t.Errorf("error %q should mention contextDir", err.Error())
	}
	if len(rec.snapshot()) != 0 {
		t.Errorf("no exec call should be made on validation failure; got %#v", rec.snapshot())
	}
}

// TestBuildImage_NoShellInterpolation asserts V5 mitigation: arguments with
// shell metacharacters are passed verbatim as separate argv elements.
// Cmder.CommandContext takes (name, args...) — there is no shell layer, so
// metacharacters cannot leak into a different argv slot.
func TestBuildImage_NoShellInterpolation(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)

	tagWithMeta := "img:latest;rm -rf /"
	if err := BuildImage(context.Background(), "docker", tagWithMeta, "/ctx"); err != nil {
		t.Fatalf("BuildImage returned error: %v", err)
	}
	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	foundTag := false
	for _, a := range calls[0].args {
		if a == tagWithMeta {
			foundTag = true
		}
		if strings.Contains(a, "rm -rf") && a != tagWithMeta {
			t.Errorf("metacharacters leaked into a different arg: %q", a)
		}
	}
	if !foundTag {
		t.Errorf("tag with metachars must appear verbatim as one argv element; args=%v", calls[0].args)
	}
}

// TestBuildImageFn_WiredToBuildImage smoke-tests the package-level injection
// point Plan 03 will call. Asserts BuildImageFn defaults to BuildImage and
// routes through devCmder.
func TestBuildImageFn_WiredToBuildImage(t *testing.T) {
	if BuildImageFn == nil {
		t.Fatal("BuildImageFn must not be nil at package init")
	}
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)
	if err := BuildImageFn(context.Background(), "docker", "x:y", "/c"); err != nil {
		t.Fatalf("BuildImageFn returned error: %v", err)
	}
	if got := len(rec.snapshot()); got != 1 {
		t.Errorf("BuildImageFn should route through devCmder; got %d calls", got)
	}
}
