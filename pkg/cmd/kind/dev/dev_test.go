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
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cmd"
	internaldev "sigs.k8s.io/kind/pkg/internal/dev"
	"sigs.k8s.io/kind/pkg/log"
)

// withRunFn swaps the package-level runFn for the duration of t and restores
// it on cleanup. Tests use this to capture Options without invoking the real
// orchestrator (matches pause_test.go pauseFn pattern).
func withRunFn(t *testing.T, fn func(opts internaldev.Options) error) {
	t.Helper()
	prev := runFn
	runFn = fn
	t.Cleanup(func() { runFn = prev })
}

// withResolveClusterName swaps the package-level resolveClusterName helper
// so tests don't need a real *cluster.Provider.
func withResolveClusterName(t *testing.T, fn func(args []string, explicit string) (string, error)) {
	t.Helper()
	prev := resolveClusterName
	resolveClusterName = fn
	t.Cleanup(func() { resolveClusterName = prev })
}

// withResolveBinaryName swaps the package-level resolveBinaryName helper.
// Tests can simulate "no container runtime" by returning "".
func withResolveBinaryName(t *testing.T, fn func() string) {
	t.Helper()
	prev := resolveBinaryName
	resolveBinaryName = fn
	t.Cleanup(func() { resolveBinaryName = prev })
}

// newTestStreams returns IOStreams with bytes.Buffer backing each stream so
// tests can inspect output.
func newTestStreams() (cmd.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return cmd.IOStreams{In: nil, Out: out, ErrOut: errOut}, out, errOut
}

// tempWatchDir returns a freshly created temp directory that is cleaned up
// when the test ends.
func tempWatchDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// installDefaultStubs wires resolveClusterName + resolveBinaryName + runFn to
// no-op success implementations. Individual tests override the pieces they
// want to assert on.
func installDefaultStubs(t *testing.T) {
	t.Helper()
	withResolveClusterName(t, func(_ []string, explicit string) (string, error) {
		if explicit != "" {
			return explicit, nil
		}
		return "kind", nil
	})
	withResolveBinaryName(t, func() string { return "docker" })
	withRunFn(t, func(_ internaldev.Options) error { return nil })
}

// TestDevCmd_RequiresWatch: invoke without --watch → cobra's required-flag
// error; assert error contains "watch" and runFn never called.
func TestDevCmd_RequiresWatch(t *testing.T) {
	installDefaultStubs(t)
	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--target=myapp"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected required-flag error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "watch") {
		t.Errorf("expected error to mention \"watch\"; got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when --watch missing")
	}
}

// TestDevCmd_RequiresTarget: --watch present but no --target → required-flag
// error.
func TestDevCmd_RequiresTarget(t *testing.T) {
	installDefaultStubs(t)
	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	dir := tempWatchDir(t)
	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected required-flag error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "target") {
		t.Errorf("expected error to mention \"target\"; got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when --target missing")
	}
}

// TestDevCmd_ValidatesWatchDir: --watch=/no/such/path → returns error
// "invalid --watch" containing the path; runFn not called.
func TestDevCmd_ValidatesWatchDir(t *testing.T) {
	installDefaultStubs(t)
	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	bogus := "/no/such/path/does-not-exist-49-04"
	c.SetArgs([]string{"--watch=" + bogus, "--target=myapp"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for non-existent watch dir, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --watch") {
		t.Errorf("expected error to contain \"invalid --watch\"; got: %v", err)
	}
	if !strings.Contains(err.Error(), bogus) {
		t.Errorf("expected error to contain the bogus path %q; got: %v", bogus, err)
	}
	if called {
		t.Errorf("runFn must not be called when --watch validation fails")
	}
}

// TestDevCmd_RejectsWatchFileNotDir: --watch points to a regular file →
// "not a directory" error.
func TestDevCmd_RejectsWatchFileNotDir(t *testing.T) {
	installDefaultStubs(t)
	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	dir := tempWatchDir(t)
	file := filepath.Join(dir, "regular-file.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + file, "--target=myapp"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for file-not-dir watch, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected error to contain \"not a directory\"; got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when --watch is a regular file")
	}
}

// TestDevCmd_FlagsPropagated: rich flag set → captured Options has each field
// populated correctly.
func TestDevCmd_FlagsPropagated(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	var captured internaldev.Options
	withRunFn(t, func(opts internaldev.Options) error {
		captured = opts
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{
		"--watch=" + dir,
		"--target=myapp",
		"--namespace=prod",
		"--image=foo:bar",
		"--debounce=200ms",
		"--poll-interval=2s",
		"--rollout-timeout=3m",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if captured.WatchDir != dir {
		t.Errorf("WatchDir: want %q got %q", dir, captured.WatchDir)
	}
	if captured.Target != "myapp" {
		t.Errorf("Target: want %q got %q", "myapp", captured.Target)
	}
	if captured.Namespace != "prod" {
		t.Errorf("Namespace: want %q got %q", "prod", captured.Namespace)
	}
	if captured.ImageTag != "foo:bar" {
		t.Errorf("ImageTag: want %q got %q", "foo:bar", captured.ImageTag)
	}
	if captured.Debounce != 200*time.Millisecond {
		t.Errorf("Debounce: want %v got %v", 200*time.Millisecond, captured.Debounce)
	}
	if captured.PollInterval != 2*time.Second {
		t.Errorf("PollInterval: want %v got %v", 2*time.Second, captured.PollInterval)
	}
	if captured.RolloutTimeout != 3*time.Minute {
		t.Errorf("RolloutTimeout: want %v got %v", 3*time.Minute, captured.RolloutTimeout)
	}
	if captured.BinaryName != "docker" {
		t.Errorf("BinaryName: want %q got %q", "docker", captured.BinaryName)
	}
	if captured.Provider == nil {
		t.Errorf("Provider must be non-nil")
	}
	if captured.Ctx == nil {
		t.Errorf("Ctx must be non-nil (cobra propagates context)")
	}
}

// TestDevCmd_PollFlag: --poll → captured Options.Poll == true.
func TestDevCmd_PollFlag(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	var captured internaldev.Options
	withRunFn(t, func(opts internaldev.Options) error {
		captured = opts
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp", "--poll"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !captured.Poll {
		t.Errorf("expected Poll=true; got false")
	}
}

// TestDevCmd_DurationFlags_RejectsBareInteger: --debounce=500 (no unit) →
// cobra parsing error (matches Phase 47-06 lesson).
func TestDevCmd_DurationFlags_RejectsBareInteger(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp", "--debounce=500"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for bare integer --debounce=500, got nil")
	}
	// cobra/pflag's DurationVar error mentions "invalid argument" + the value;
	// time.ParseDuration explicitly says "missing unit in duration".
	msg := err.Error()
	if !strings.Contains(msg, "missing unit") && !strings.Contains(msg, "invalid argument") {
		t.Errorf("expected duration-parse error, got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when flag parse fails")
	}
}

// TestDevCmd_DurationFlags_AcceptsSuffix: --debounce=750ms → Options.Debounce
// == 750ms.
func TestDevCmd_DurationFlags_AcceptsSuffix(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	var captured internaldev.Options
	withRunFn(t, func(opts internaldev.Options) error {
		captured = opts
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp", "--debounce=750ms"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if captured.Debounce != 750*time.Millisecond {
		t.Errorf("Debounce: want %v got %v", 750*time.Millisecond, captured.Debounce)
	}
}

// TestDevCmd_PropagatesError: fake runFn returns errors.New("watch failed");
// RunE returns same error.
func TestDevCmd_PropagatesError(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	want := errors.New("watch failed")
	withRunFn(t, func(_ internaldev.Options) error { return want })

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected propagated error, got nil")
	}
	if !errors.Is(err, want) {
		t.Errorf("expected wrapped %v; got: %v", want, err)
	}
}

// TestDevCmd_DefaultsApplied: only --watch and --target set; assert
// Options.Namespace=="default", Debounce==500ms, Poll==false, PollInterval==1s,
// RolloutTimeout==2m, ImageTag empty (Run will derive default).
func TestDevCmd_DefaultsApplied(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	var captured internaldev.Options
	withRunFn(t, func(opts internaldev.Options) error {
		captured = opts
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if captured.Namespace != "default" {
		t.Errorf("Namespace default: want %q got %q", "default", captured.Namespace)
	}
	if captured.Debounce != 500*time.Millisecond {
		t.Errorf("Debounce default: want %v got %v", 500*time.Millisecond, captured.Debounce)
	}
	if captured.Poll {
		t.Errorf("Poll default: want false got true")
	}
	if captured.PollInterval != time.Second {
		t.Errorf("PollInterval default: want %v got %v", time.Second, captured.PollInterval)
	}
	if captured.RolloutTimeout != 2*time.Minute {
		t.Errorf("RolloutTimeout default: want %v got %v", 2*time.Minute, captured.RolloutTimeout)
	}
	if captured.ImageTag != "" {
		t.Errorf("ImageTag default at CLI layer: want \"\" (Run derives kinder-dev/<target>:latest); got %q", captured.ImageTag)
	}
}

// TestDevCmd_AutoDetectsCluster: --name unset; injected resolveClusterName
// returns ("auto-found", nil); assert Options.ClusterName == "auto-found".
func TestDevCmd_AutoDetectsCluster(t *testing.T) {
	dir := tempWatchDir(t)
	withResolveBinaryName(t, func() string { return "docker" })
	withResolveClusterName(t, func(args []string, explicit string) (string, error) {
		if explicit != "" {
			t.Errorf("expected empty explicit, got %q", explicit)
		}
		if len(args) != 0 {
			t.Errorf("expected empty positional args, got %v", args)
		}
		return "auto-found", nil
	})

	var captured internaldev.Options
	withRunFn(t, func(opts internaldev.Options) error {
		captured = opts
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if captured.ClusterName != "auto-found" {
		t.Errorf("ClusterName: want %q got %q", "auto-found", captured.ClusterName)
	}
}

// TestDevCmd_ExplicitName: --name=mycluster; injected resolveClusterName
// receives explicit="mycluster" and returns it.
func TestDevCmd_ExplicitName(t *testing.T) {
	dir := tempWatchDir(t)
	withResolveBinaryName(t, func() string { return "docker" })
	gotExplicit := ""
	withResolveClusterName(t, func(args []string, explicit string) (string, error) {
		gotExplicit = explicit
		return explicit, nil
	})

	var captured internaldev.Options
	withRunFn(t, func(opts internaldev.Options) error {
		captured = opts
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp", "--name=mycluster"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if gotExplicit != "mycluster" {
		t.Errorf("explicit: want %q got %q", "mycluster", gotExplicit)
	}
	if captured.ClusterName != "mycluster" {
		t.Errorf("ClusterName: want %q got %q", "mycluster", captured.ClusterName)
	}
}

// TestDevCmd_ResolveBinaryError: injected resolveBinaryName returns "" (no
// runtime found); RunE returns error mentioning "container runtime".
func TestDevCmd_ResolveBinaryError(t *testing.T) {
	dir := tempWatchDir(t)
	withResolveClusterName(t, func(_ []string, _ string) (string, error) { return "kind", nil })
	withResolveBinaryName(t, func() string { return "" })
	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error when no provider binary found, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "container runtime") &&
		!strings.Contains(strings.ToLower(err.Error()), "provider binary") {
		t.Errorf("expected error to mention \"container runtime\" or \"provider binary\"; got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when binary resolution fails")
	}
}

// TestDevCmd_NegativeDebounceRejected: --debounce=-1s → command exits non-zero
// with explicit message; runFn not called.
func TestDevCmd_NegativeDebounceRejected(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp", "--debounce=-1s"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for negative --debounce, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --debounce") {
		t.Errorf("expected error to contain \"invalid --debounce\"; got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when --debounce validation fails")
	}
}

// TestDevCmd_ZeroPollIntervalRejected: --poll-interval=0s → exits non-zero.
func TestDevCmd_ZeroPollIntervalRejected(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp", "--poll-interval=0s"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for --poll-interval=0s, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --poll-interval") {
		t.Errorf("expected error to contain \"invalid --poll-interval\"; got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when --poll-interval validation fails")
	}
}

// TestDevCmd_ZeroRolloutTimeoutRejected: --rollout-timeout=0s → exits non-zero.
func TestDevCmd_ZeroRolloutTimeoutRejected(t *testing.T) {
	installDefaultStubs(t)
	dir := tempWatchDir(t)

	called := false
	withRunFn(t, func(_ internaldev.Options) error {
		called = true
		return nil
	})

	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--watch=" + dir, "--target=myapp", "--rollout-timeout=0s"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for --rollout-timeout=0s, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --rollout-timeout") {
		t.Errorf("expected error to contain \"invalid --rollout-timeout\"; got: %v", err)
	}
	if called {
		t.Errorf("runFn must not be called when --rollout-timeout validation fails")
	}
}

// TestDevCmd_HelpListsCriticalFlags: `kinder dev --help` output contains the
// four critical flags advertised by RESEARCH SC1. Sanity check that the
// command is registered and its Long/help text is non-empty.
//
// Note: cobra writes --help to the writer set via SetOut. In production
// pkg/cmd/kind/root.go calls SetOut on the root command and children
// inherit. In this isolated test we have to set it on the dev command
// directly because we instantiate it without a parent.
func TestDevCmd_HelpListsCriticalFlags(t *testing.T) {
	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	helpBuf := &bytes.Buffer{}
	c.SetOut(helpBuf)
	c.SetArgs([]string{"--help"})
	if err := c.Execute(); err != nil {
		t.Fatalf("--help exited error: %v", err)
	}
	out := helpBuf.String()
	for _, want := range []string{"--watch", "--target", "--debounce", "--poll"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected --help to mention %q; got:\n%s", want, out)
		}
	}
}
