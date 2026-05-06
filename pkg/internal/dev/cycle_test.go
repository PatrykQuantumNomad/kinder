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
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

// --- helpers shared with dev_test.go ---

// withBuildImageFn swaps the package-level BuildImageFn for the duration
// of the test, restoring the original on cleanup.
func withBuildImageFn(t *testing.T, fn func(ctx context.Context, binaryName, imageTag, contextDir string) error) {
	t.Helper()
	prev := BuildImageFn
	BuildImageFn = fn
	t.Cleanup(func() { BuildImageFn = prev })
}

// withRolloutFn swaps the package-level RolloutFn for the duration of the test.
func withRolloutFn(t *testing.T, fn func(ctx context.Context, kubeconfigPath, namespace, deployment string, timeout time.Duration) error) {
	t.Helper()
	prev := RolloutFn
	RolloutFn = fn
	t.Cleanup(func() { RolloutFn = prev })
}

// fakeLoader is a minimal LoadOptions.ImageLoaderFn that records the call
// count and returns a configurable error. It bypasses the real
// LoadImagesIntoCluster pipeline by making the cycle short-circuit before
// reaching the production load path.
//
// We use a package-level injection: tests swap loadImagesFn (declared on
// cycle.go) to control LoadImagesIntoCluster behavior without setting up a
// fake provider/nodeLister/etc.

// withLoadImagesFn swaps the package-level loadImagesFn used by runOneCycle.
func withLoadImagesFn(t *testing.T, fn func(ctx context.Context, opts LoadOptions) error) {
	t.Helper()
	prev := loadImagesFn
	loadImagesFn = fn
	t.Cleanup(func() { loadImagesFn = prev })
}

// newCycleStreams returns a cmd.IOStreams whose Out and ErrOut are
// bytes.Buffers the test can inspect, plus references to those buffers.
func newCycleStreams() (cmd.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return cmd.IOStreams{In: strings.NewReader(""), Out: out, ErrOut: errOut}, out, errOut
}

// baseCycleOpts builds a cycleOpts with all required scalar fields set; the
// test provides streams.
func baseCycleOpts(streams cmd.IOStreams, cycleNum int) cycleOpts {
	return cycleOpts{
		cycleNum:       cycleNum,
		binaryName:     "docker",
		imageTag:       "kinder-dev/myapp:latest",
		watchDir:       "/ctx",
		namespace:      "default",
		target:         "myapp",
		rolloutTimeout: 90 * time.Second,
		kubeconfigPath: "/tmp/kc",
		clusterName:    "kinder",
		provider:       nil, // tests use loadImagesFn injection so Provider is unused
		logger:         log.NoopLogger{},
		streams:        streams,
	}
}

// --- runOneCycle tests ---

func TestRunOneCycle_HappyPath(t *testing.T) {
	var buildCalls, loadCalls, rolloutCalls int32

	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error {
		atomic.AddInt32(&buildCalls, 1)
		return nil
	})
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error {
		atomic.AddInt32(&loadCalls, 1)
		return nil
	})
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error {
		atomic.AddInt32(&rolloutCalls, 1)
		return nil
	})

	streams, out, _ := newCycleStreams()
	if err := runOneCycle(context.Background(), baseCycleOpts(streams, 7)); err != nil {
		t.Fatalf("runOneCycle returned error: %v", err)
	}

	if atomic.LoadInt32(&buildCalls) != 1 {
		t.Errorf("BuildImageFn called %d times, want 1", buildCalls)
	}
	if atomic.LoadInt32(&loadCalls) != 1 {
		t.Errorf("loadImagesFn called %d times, want 1", loadCalls)
	}
	if atomic.LoadInt32(&rolloutCalls) != 1 {
		t.Errorf("RolloutFn called %d times, want 1", rolloutCalls)
	}

	got := out.String()
	wantSubs := []string{
		"[cycle 7] Change detected",
		"build:",
		"load:",
		"rollout:",
		"total:",
	}
	prevIdx := -1
	for _, sub := range wantSubs {
		idx := strings.Index(got, sub)
		if idx < 0 {
			t.Errorf("output missing %q. Full output:\n%s", sub, got)
			continue
		}
		if idx <= prevIdx {
			t.Errorf("substring %q at %d came before previous substring at %d. Full output:\n%s", sub, idx, prevIdx, got)
		}
		prevIdx = idx
	}
}

func TestRunOneCycle_BuildFails(t *testing.T) {
	var loadCalls, rolloutCalls int32

	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error {
		return errors.New("docker boom")
	})
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error {
		atomic.AddInt32(&loadCalls, 1)
		return nil
	})
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error {
		atomic.AddInt32(&rolloutCalls, 1)
		return nil
	})

	streams, out, _ := newCycleStreams()
	err := runOneCycle(context.Background(), baseCycleOpts(streams, 1))
	if err == nil {
		t.Fatal("runOneCycle should have returned an error")
	}
	if !strings.Contains(err.Error(), "build") {
		t.Errorf("error %q should contain 'build'", err.Error())
	}
	if !strings.Contains(err.Error(), "docker boom") {
		t.Errorf("error %q should contain 'docker boom'", err.Error())
	}

	if atomic.LoadInt32(&loadCalls) != 0 {
		t.Errorf("loadImagesFn must NOT be called when build fails; got %d calls", loadCalls)
	}
	if atomic.LoadInt32(&rolloutCalls) != 0 {
		t.Errorf("RolloutFn must NOT be called when build fails; got %d calls", rolloutCalls)
	}

	got := out.String()
	if !strings.Contains(got, "build:") {
		t.Errorf("output should print build line. Got:\n%s", got)
	}
	if !strings.Contains(got, "(error:") {
		t.Errorf("output should annotate failed build with '(error:'. Got:\n%s", got)
	}
	if strings.Contains(got, "load:") {
		t.Errorf("output must NOT print load line when build failed. Got:\n%s", got)
	}
	if strings.Contains(got, "rollout:") {
		t.Errorf("output must NOT print rollout line when build failed. Got:\n%s", got)
	}
}

func TestRunOneCycle_LoadFails(t *testing.T) {
	var rolloutCalls int32

	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error { return nil })
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error {
		return errors.New("image load exploded")
	})
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error {
		atomic.AddInt32(&rolloutCalls, 1)
		return nil
	})

	streams, out, _ := newCycleStreams()
	err := runOneCycle(context.Background(), baseCycleOpts(streams, 2))
	if err == nil {
		t.Fatal("runOneCycle should have returned an error")
	}
	if !strings.Contains(err.Error(), "load") {
		t.Errorf("error %q should contain 'load'", err.Error())
	}
	if !strings.Contains(err.Error(), "image load exploded") {
		t.Errorf("error %q should wrap underlying %q", err.Error(), "image load exploded")
	}
	if atomic.LoadInt32(&rolloutCalls) != 0 {
		t.Errorf("RolloutFn must NOT be called when load fails; got %d calls", rolloutCalls)
	}

	got := out.String()
	if !strings.Contains(got, "load:") {
		t.Errorf("output should print load line. Got:\n%s", got)
	}
	if !strings.Contains(got, "(error:") {
		t.Errorf("output should annotate failed load with '(error:'. Got:\n%s", got)
	}
	if strings.Contains(got, "rollout:") {
		t.Errorf("output must NOT print rollout line when load failed. Got:\n%s", got)
	}
}

func TestRunOneCycle_RolloutFails(t *testing.T) {
	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error { return nil })
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error { return nil })
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error {
		return errors.New("rollout timeout")
	})

	streams, out, _ := newCycleStreams()
	err := runOneCycle(context.Background(), baseCycleOpts(streams, 3))
	if err == nil {
		t.Fatal("runOneCycle should have returned an error")
	}
	if !strings.Contains(err.Error(), "rollout") {
		t.Errorf("error %q should contain 'rollout'", err.Error())
	}

	got := out.String()
	if !strings.Contains(got, "rollout:") {
		t.Errorf("output should print rollout line. Got:\n%s", got)
	}
	if !strings.Contains(got, "(error:") {
		t.Errorf("output should annotate failed rollout with '(error:'. Got:\n%s", got)
	}
	if strings.Contains(got, "total:") {
		t.Errorf("output must NOT print total line when cycle failed. Got:\n%s", got)
	}
}

// TestRunOneCycle_TimingOutput asserts the per-step timing lines all match
// the canonical Phase 47/48 format `<step>:   X.Xs` (one decimal, "s" suffix).
func TestRunOneCycle_TimingOutput(t *testing.T) {
	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error {
		time.Sleep(20 * time.Millisecond) // small sleep to ensure non-zero
		return nil
	})
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error { return nil })
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error { return nil })

	streams, out, _ := newCycleStreams()
	if err := runOneCycle(context.Background(), baseCycleOpts(streams, 5)); err != nil {
		t.Fatalf("runOneCycle returned error: %v", err)
	}

	// Each timing line: leading two-space indent, label, whitespace, then
	// `<digits>.<digit>s`.
	stepLine := regexp.MustCompile(`(?m)^  (build|load|rollout|total):\s+\d+\.\ds$`)
	matches := stepLine.FindAllString(out.String(), -1)
	if len(matches) != 4 {
		t.Errorf("expected 4 timing lines (build, load, rollout, total) matching `%s`; got %d:\n%s",
			stepLine.String(), len(matches), out.String())
	}
}

// TestRunOneCycle_HeaderPrintedBeforeWork asserts the "[cycle N] Change
// detected" banner is emitted *before* any build step runs. The fake build
// observes the streams.Out buffer at invocation time.
func TestRunOneCycle_HeaderPrintedBeforeWork(t *testing.T) {
	streams, out, _ := newCycleStreams()

	var observedAtBuild string
	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error {
		observedAtBuild = out.String()
		return nil
	})
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error { return nil })
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error { return nil })

	if err := runOneCycle(context.Background(), baseCycleOpts(streams, 11)); err != nil {
		t.Fatalf("runOneCycle returned error: %v", err)
	}
	if !strings.Contains(observedAtBuild, "[cycle 11] Change detected") {
		t.Errorf("header should be printed before build runs; build saw out=%q", observedAtBuild)
	}
}

// TestRunOneCycle_NilStreamsDoesNotPanic asserts a defensive guard: if a
// caller hands us a partially-initialized cycleOpts (Out == nil), runOneCycle
// must not panic on the first Fprintf. We expect runOneCycle to substitute
// io.Discard or the equivalent when Out is nil. This protects against
// programmer error in Plan 04's CLI wiring.
func TestRunOneCycle_NilStreamsDoesNotPanic(t *testing.T) {
	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error { return nil })
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error { return nil })
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error { return nil })

	// streams.Out is nil deliberately
	opts := baseCycleOpts(cmd.IOStreams{In: strings.NewReader(""), Out: nil, ErrOut: io.Discard}, 1)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runOneCycle panicked with nil Out: %v", r)
		}
	}()
	if err := runOneCycle(context.Background(), opts); err != nil {
		t.Errorf("runOneCycle returned error: %v", err)
	}
}
