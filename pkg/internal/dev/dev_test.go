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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

// withKubeconfigGetter is declared in kubeconfig_test.go (Plan 49-02). We
// reuse it here without redeclaration.

// runHarness bundles the wired-up Options + the test buffers + a few
// counters tests inspect to assert orchestrator behavior.
type runHarness struct {
	opts       Options
	out, errOut *bytes.Buffer
	buildCalls *int32
	loadCalls  *int32
	rollCalls  *int32
	events     chan struct{}
	cancel     context.CancelFunc
}

// newRunHarness wires up Options with no-op fakes for build/load/rollout,
// a bytes.Buffer-backed cmd.IOStreams, an injected EventSource, and a
// canned kubeconfigGetter. Returns the harness ready to call Run.
//
// Tests typically:
//  1. Call newRunHarness(t).
//  2. Optionally swap fakes (withBuildImageFn, withLoadImagesFn, withRolloutFn).
//  3. Spawn `go func() { errCh <- Run(h.opts) }()`.
//  4. Send events on h.events.
//  5. Cancel ctx via h.cancel(); wait on errCh.
func newRunHarness(t *testing.T) *runHarness {
	t.Helper()

	var buildCalls, loadCalls, rollCalls int32
	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error {
		atomic.AddInt32(&buildCalls, 1)
		return nil
	})
	withLoadImagesFn(t, func(_ context.Context, _ LoadOptions) error {
		atomic.AddInt32(&loadCalls, 1)
		return nil
	})
	withRolloutFn(t, func(_ context.Context, _, _, _ string, _ time.Duration) error {
		atomic.AddInt32(&rollCalls, 1)
		return nil
	})
	withKubeconfigGetter(t, func(_ *cluster.Provider, _ string, _ bool) (string, error) {
		return "fake-kubeconfig\n", nil
	})

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	streams := cmd.IOStreams{In: strings.NewReader(""), Out: out, ErrOut: errOut}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	events := make(chan struct{}, 32)
	// Provider is intentionally a non-nil sentinel — it is passed to
	// kubeconfigGetter (faked) and never dereferenced by tests. We use
	// &cluster.Provider{} default-zero so callers don't need internals.
	prov := &cluster.Provider{}

	opts := Options{
		Ctx:            ctx,
		ClusterName:    "kinder",
		Provider:       prov,
		BinaryName:     "docker",
		WatchDir:       "/tmp/ctx",
		Target:         "myapp",
		Namespace:      "default",
		ImageTag:       "kinder-dev/myapp:latest",
		Debounce:       50 * time.Millisecond,
		PollInterval:   time.Second,
		RolloutTimeout: 90 * time.Second,
		Logger:         log.NoopLogger{},
		Streams:        streams,
		EventSource:    events,
		SkipBanner:     false,
	}
	return &runHarness{
		opts:       opts,
		out:        out,
		errOut:     errOut,
		buildCalls: &buildCalls,
		loadCalls:  &loadCalls,
		rollCalls:  &rollCalls,
		events:     events,
		cancel:     cancel,
	}
}

// startRun launches Run on a goroutine and returns a channel the test waits
// on for Run's return value.
func startRun(h *runHarness) <-chan error {
	errCh := make(chan error, 1)
	go func() { errCh <- Run(h.opts) }()
	return errCh
}

// waitErr drains errCh with a timeout; fails the test on timeout.
func waitErr(t *testing.T, errCh <-chan error, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		t.Fatalf("Run did not return within %s", timeout)
		return nil
	}
}

// --- tests ---

func TestRun_BannerPrinted(t *testing.T) {
	h := newRunHarness(t)
	errCh := startRun(h)
	// Give the goroutine a moment to print the banner before we cancel.
	time.Sleep(20 * time.Millisecond)
	h.cancel()
	if err := waitErr(t, errCh, 500*time.Millisecond); err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	got := h.out.String()
	wantSubs := []string{
		"Watching /tmp/ctx",
		"deployment/myapp",
		"cluster: kinder",
		"namespace: default",
		"Debounce: 50ms",
		"Mode: fsnotify",
		"Press Ctrl+C",
	}
	for _, sub := range wantSubs {
		if !strings.Contains(got, sub) {
			t.Errorf("banner missing %q. Full output:\n%s", sub, got)
		}
	}
}

func TestRun_BannerPollMode(t *testing.T) {
	h := newRunHarness(t)
	h.opts.Poll = true

	errCh := startRun(h)
	time.Sleep(20 * time.Millisecond)
	h.cancel()
	if err := waitErr(t, errCh, 500*time.Millisecond); err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	if !strings.Contains(h.out.String(), "Mode: poll") {
		t.Errorf("expected 'Mode: poll' in output for Poll=true; got:\n%s", h.out.String())
	}
}

func TestRun_TriggersOnEvent(t *testing.T) {
	h := newRunHarness(t)
	errCh := startRun(h)

	// Fire one event; wait for debounce + cycle to complete.
	h.events <- struct{}{}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(h.buildCalls) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt32(h.buildCalls); got != 1 {
		t.Errorf("expected exactly 1 build call after one event; got %d", got)
	}

	h.cancel()
	if err := waitErr(t, errCh, 500*time.Millisecond); err != nil {
		t.Errorf("Run returned error: %v", err)
	}
	if !strings.Contains(h.out.String(), "[cycle 1] Change detected") {
		t.Errorf("expected '[cycle 1] Change detected' in output; got:\n%s", h.out.String())
	}
}

func TestRun_DebouncesBurst(t *testing.T) {
	h := newRunHarness(t)
	errCh := startRun(h)

	// Send 10 events immediately; debounce window is 50ms.
	for i := 0; i < 10; i++ {
		h.events <- struct{}{}
	}
	// Wait for debounce + grace.
	time.Sleep(150 * time.Millisecond)

	if got := atomic.LoadInt32(h.buildCalls); got != 1 {
		t.Errorf("burst of 10 events should debounce to 1 cycle; got %d builds", got)
	}

	h.cancel()
	if err := waitErr(t, errCh, 500*time.Millisecond); err != nil {
		t.Errorf("Run returned error: %v", err)
	}
}

// TestRun_ConcurrentCyclesPrevented verifies RESEARCH common pitfall 3.
// While a cycle is running, additional events arrive. The cycle runner must
// drain those events non-blockingly so we end up with exactly two cycles
// (one for the initial trigger, one for the burst that happened during the
// in-flight cycle). Critically, NOT 11 cycles.
func TestRun_ConcurrentCyclesPrevented(t *testing.T) {
	h := newRunHarness(t)

	// Override BuildImageFn with one that blocks on a release channel,
	// simulating a slow build.
	release := make(chan struct{})
	var buildSeen int32
	withBuildImageFn(t, func(ctx context.Context, _, _, _ string) error {
		atomic.AddInt32(&buildSeen, 1)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	errCh := startRun(h)

	// Trigger cycle 1.
	h.events <- struct{}{}
	// Wait for build to start.
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) && atomic.LoadInt32(&buildSeen) < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if atomic.LoadInt32(&buildSeen) != 1 {
		t.Fatalf("first build never started; buildSeen=%d", atomic.LoadInt32(&buildSeen))
	}

	// While cycle 1 is blocked, send 5 more events.
	for i := 0; i < 5; i++ {
		h.events <- struct{}{}
	}
	// Give the debouncer time to process the burst (single emit collapses).
	time.Sleep(100 * time.Millisecond)

	// Release cycle 1.
	close(release)
	// Wait for cycle 2 to fire (debounce + grace after release).
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && atomic.LoadInt32(&buildSeen) < 2 {
		time.Sleep(5 * time.Millisecond)
	}

	got := atomic.LoadInt32(&buildSeen)
	if got != 2 {
		t.Errorf("expected exactly 2 builds (initial + post-burst); got %d. RESEARCH common pitfall 3 may be violated.", got)
	}

	h.cancel()
	if err := waitErr(t, errCh, 1*time.Second); err != nil {
		t.Errorf("Run returned error: %v", err)
	}
}

func TestRun_CtxCancelExits(t *testing.T) {
	h := newRunHarness(t)
	errCh := startRun(h)

	// Cancel immediately.
	time.Sleep(20 * time.Millisecond) // allow Run to set up
	startCancel := time.Now()
	h.cancel()
	if err := waitErr(t, errCh, 500*time.Millisecond); err != nil {
		t.Errorf("Run returned error on ctx-cancel: %v", err)
	}
	if d := time.Since(startCancel); d > 200*time.Millisecond {
		t.Errorf("Run took %s to exit after ctx-cancel; want <200ms", d)
	}
}

func TestRun_RejectsMissingWatchDir(t *testing.T) {
	h := newRunHarness(t)
	h.opts.WatchDir = ""

	err := Run(h.opts)
	if err == nil {
		t.Fatal("expected error for empty WatchDir")
	}
	if !strings.Contains(err.Error(), "watch") {
		t.Errorf("error %q should mention 'watch'", err.Error())
	}
	// Banner / cycle output must NOT be produced.
	if strings.Contains(h.out.String(), "Watching") {
		t.Errorf("validation must short-circuit BEFORE banner; got out:\n%s", h.out.String())
	}
}

func TestRun_RejectsMissingTarget(t *testing.T) {
	h := newRunHarness(t)
	h.opts.Target = ""

	err := Run(h.opts)
	if err == nil {
		t.Fatal("expected error for empty Target")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("error %q should mention 'target'", err.Error())
	}
}

func TestRun_RejectsMissingClusterName(t *testing.T) {
	h := newRunHarness(t)
	h.opts.ClusterName = ""

	err := Run(h.opts)
	if err == nil {
		t.Fatal("expected error for empty ClusterName")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "clustername") {
		t.Errorf("error %q should mention ClusterName", err.Error())
	}
}

func TestRun_RejectsMissingProvider(t *testing.T) {
	h := newRunHarness(t)
	h.opts.Provider = nil

	err := Run(h.opts)
	if err == nil {
		t.Fatal("expected error for nil Provider")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "provider") {
		t.Errorf("error %q should mention Provider", err.Error())
	}
}

func TestRun_RejectsMissingBinaryName(t *testing.T) {
	h := newRunHarness(t)
	h.opts.BinaryName = ""

	err := Run(h.opts)
	if err == nil {
		t.Fatal("expected error for empty BinaryName")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "binaryname") {
		t.Errorf("error %q should mention BinaryName", err.Error())
	}
}

// TestRun_DefaultImageTag asserts that when ImageTag is empty, Run defaults
// it to "kinder-dev/<target>:latest" and the build fake sees that tag.
func TestRun_DefaultImageTag(t *testing.T) {
	h := newRunHarness(t)
	h.opts.ImageTag = ""
	h.opts.Target = "myservice"

	var seenTag atomic.Value
	withBuildImageFn(t, func(_ context.Context, _, imageTag, _ string) error {
		seenTag.Store(imageTag)
		return nil
	})

	errCh := startRun(h)
	h.events <- struct{}{}
	// Wait for cycle to fire.
	time.Sleep(150 * time.Millisecond)

	h.cancel()
	if err := waitErr(t, errCh, 500*time.Millisecond); err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	got, _ := seenTag.Load().(string)
	want := "kinder-dev/myservice:latest"
	if got != want {
		t.Errorf("imageTag default = %q, want %q", got, want)
	}
}

// TestRun_ExitOnFirstError asserts that with ExitOnFirstError=true, Run
// returns the first cycle's error without continuing.
func TestRun_ExitOnFirstError(t *testing.T) {
	h := newRunHarness(t)
	h.opts.ExitOnFirstError = true

	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error {
		return errors.New("boom")
	})

	errCh := startRun(h)
	h.events <- struct{}{}

	err := waitErr(t, errCh, 1*time.Second)
	if err == nil {
		t.Fatal("expected Run to return cycle error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q should wrap 'boom'", err.Error())
	}
}

// TestRun_DefaultErrorBehaviorContinues asserts that with the default
// (ExitOnFirstError=false), Run continues watching after a failed cycle and
// the next event triggers another cycle. Run's eventual return value is the
// FIRST cycle's error.
func TestRun_DefaultErrorBehaviorContinues(t *testing.T) {
	h := newRunHarness(t)

	var cycleNum int32
	var mu sync.Mutex
	var errs []error
	withBuildImageFn(t, func(_ context.Context, _, _, _ string) error {
		n := atomic.AddInt32(&cycleNum, 1)
		mu.Lock()
		defer mu.Unlock()
		if n == 1 {
			err := errors.New("first-cycle-boom")
			errs = append(errs, err)
			return err
		}
		errs = append(errs, nil)
		return nil
	})

	errCh := startRun(h)

	// Cycle 1: fails.
	h.events <- struct{}{}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && atomic.LoadInt32(&cycleNum) < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if atomic.LoadInt32(&cycleNum) != 1 {
		t.Fatalf("cycle 1 never ran; cycleNum=%d", atomic.LoadInt32(&cycleNum))
	}
	// Wait a tick for cycle 1 to complete and runner to drain.
	time.Sleep(100 * time.Millisecond)

	// Cycle 2: succeeds.
	h.events <- struct{}{}
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && atomic.LoadInt32(&cycleNum) < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	if atomic.LoadInt32(&cycleNum) != 2 {
		t.Errorf("cycle 2 should have run after cycle 1 failed; cycleNum=%d", atomic.LoadInt32(&cycleNum))
	}

	h.cancel()
	err := waitErr(t, errCh, 1*time.Second)
	if err == nil {
		t.Fatal("Run should have returned the first cycle's error")
	}
	if !strings.Contains(err.Error(), "first-cycle-boom") {
		t.Errorf("returned error %q should contain first-cycle's message", err.Error())
	}

	// ErrOut should mention cycle 1 failure.
	if !strings.Contains(h.errOut.String(), "cycle 1") {
		t.Errorf("ErrOut should log cycle 1 failure; got:\n%s", h.errOut.String())
	}
}

// TestRun_KubeconfigWrittenOnceNotPerCycle asserts RESEARCH anti-pattern:
// kubeconfigGetter is invoked exactly once at startup, NOT per-cycle.
func TestRun_KubeconfigWrittenOnceNotPerCycle(t *testing.T) {
	h := newRunHarness(t)
	var getterCalls int32
	withKubeconfigGetter(t, func(_ *cluster.Provider, _ string, _ bool) (string, error) {
		atomic.AddInt32(&getterCalls, 1)
		return "fake-kubeconfig\n", nil
	})

	errCh := startRun(h)
	// Trigger 3 cycles separated by debounce.
	for i := 0; i < 3; i++ {
		h.events <- struct{}{}
		time.Sleep(120 * time.Millisecond)
	}

	h.cancel()
	if err := waitErr(t, errCh, 1*time.Second); err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	if got := atomic.LoadInt32(&getterCalls); got != 1 {
		t.Errorf("kubeconfigGetter called %d times; expected exactly 1 (RESEARCH anti-pattern: kubeconfig is per-Run, not per-cycle)", got)
	}
}

// TestRun_KubeconfigErrorPropagates asserts WriteKubeconfigTemp failure
// surfaces as Run's return value (and no banner / cycles run).
func TestRun_KubeconfigErrorPropagates(t *testing.T) {
	h := newRunHarness(t)
	withKubeconfigGetter(t, func(_ *cluster.Provider, _ string, _ bool) (string, error) {
		return "", errors.New("kubeconfig boom")
	})

	err := Run(h.opts)
	if err == nil {
		t.Fatal("expected Run to return kubeconfig error")
	}
	if !strings.Contains(err.Error(), "kubeconfig") {
		t.Errorf("error %q should mention kubeconfig", err.Error())
	}
	// Banner already printed by this point — that's fine; the assertion is
	// that no cycle ran.
	if got := atomic.LoadInt32(h.buildCalls); got != 0 {
		t.Errorf("no cycle should run when kubeconfig fails; got %d builds", got)
	}
}

// TestRun_DefaultsApplied asserts the documented defaults populate when
// fields are zero-valued.
func TestRun_DefaultsApplied(t *testing.T) {
	h := newRunHarness(t)
	h.opts.Namespace = ""
	h.opts.Debounce = 0
	h.opts.PollInterval = 0
	h.opts.RolloutTimeout = 0

	var seenTimeout atomic.Value
	withRolloutFn(t, func(_ context.Context, _, ns, _ string, timeout time.Duration) error {
		// Confirm namespace default is applied.
		if ns != "default" {
			t.Errorf("namespace default = %q, want %q", ns, "default")
		}
		seenTimeout.Store(timeout)
		return nil
	})

	errCh := startRun(h)
	h.events <- struct{}{}
	// Default debounce is 500ms, so wait long enough.
	time.Sleep(800 * time.Millisecond)

	h.cancel()
	if err := waitErr(t, errCh, 1*time.Second); err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	got, _ := seenTimeout.Load().(time.Duration)
	if got != 2*time.Minute {
		t.Errorf("rolloutTimeout default = %s, want 2m", got)
	}
}
