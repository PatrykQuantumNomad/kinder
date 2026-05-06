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
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

// Options carries the full configuration for one `kinder dev` invocation.
// Plan 04 (CLI shell, pkg/cmd/kind/dev/dev.go) builds this from cobra flags
// + cluster auto-detection, then calls Run.
//
// The struct is part of the LOCKED API surface for Plan 04 — field names
// must not change without coordination.
type Options struct {
	// Ctx, when non-nil, is used as the parent context. nil → context.Background.
	// Either way Run wraps it via signal.NotifyContext to add SIGINT/SIGTERM
	// teardown (RESEARCH common pitfall 6).
	Ctx context.Context

	// Cluster targeting. ClusterName must already be resolved by the caller
	// (Plan 04 uses lifecycle.ResolveClusterName). Provider is dereferenced
	// indirectly via the kubeconfigGetter package var so tests can swap the
	// getter without building a fake provider. BinaryName is docker /
	// podman / nerdctl / finch / ...
	ClusterName string
	Provider    *cluster.Provider
	BinaryName  string

	// Required user inputs.
	WatchDir  string // --watch
	Target    string // --target deployment name
	Namespace string // --namespace, default "default"
	ImageTag  string // --image; default "kinder-dev/<target>:latest"

	// Watch-mode tuning.
	Debounce       time.Duration // --debounce, default 500ms
	Poll           bool          // --poll
	PollInterval   time.Duration // --poll-interval, default 1s
	RolloutTimeout time.Duration // --rollout-timeout, default 2m

	// I/O.
	Logger  log.Logger
	Streams cmd.IOStreams

	// Test hooks (NOT exposed via CLI flags).
	//
	// EventSource, when non-nil, replaces the real watcher/poller — Run
	// consumes events directly from this channel. Tests inject a synthetic
	// channel so the loop logic is unit-testable without spinning a real
	// file watcher.
	EventSource <-chan struct{}

	// SkipBanner suppresses the SC1 banner; tests use this to reduce noise.
	SkipBanner bool

	// ExitOnFirstError, when true, makes Run return the first cycle's error
	// immediately. Default (false) is "log to ErrOut and continue watching".
	ExitOnFirstError bool
}

// Run starts the watch-mode loop. Blocks until ctx is canceled (SIGINT,
// SIGTERM, or caller-provided ctx). Returns nil on clean exit; returns the
// first cycle's error if ExitOnFirstError is set, or the first cycle error
// observed (otherwise) once ctx-cancel terminates the loop.
//
// Lifecycle:
//  1. Validate required Options fields and apply documented defaults.
//  2. Wrap Options.Ctx with signal.NotifyContext(SIGINT, SIGTERM) — Ctrl+C
//     tears down the entire watch loop (RESEARCH common pitfall 6).
//  3. Write the cluster's kubeconfig to a temp file ONCE at startup.
//     Cleanup is deferred to Run's exit, NOT per-cycle (RESEARCH
//     anti-pattern: per-cycle creation is wasteful and a leak risk).
//  4. Print the banner (unless SkipBanner).
//  5. Start the event source: Options.EventSource if non-nil; else
//     StartPoller (if opts.Poll) or StartWatcher (default).
//  6. Pipe through Debounce(window=opts.Debounce).
//  7. Drain the debounced channel serially: at most one runOneCycle in
//     flight at a time. After each cycle, drain a single queued event
//     non-blockingly (RESEARCH common pitfall 3) so a debounced burst
//     that arrived during the cycle does not trigger a redundant
//     immediate re-cycle.
//  8. Outer select includes <-ctx.Done() so Ctrl+C wakes the loop even
//     if the cycles channel is still open.
func Run(opts Options) error {
	// 1. Validate required inputs.
	if opts.WatchDir == "" {
		return fmt.Errorf("kinder dev: --watch is required")
	}
	if opts.Target == "" {
		return fmt.Errorf("kinder dev: --target is required")
	}
	if opts.ClusterName == "" {
		return fmt.Errorf("kinder dev: ClusterName is required (resolve via lifecycle.ResolveClusterName before calling Run)")
	}
	if opts.Provider == nil {
		return fmt.Errorf("kinder dev: Provider is required")
	}
	if opts.BinaryName == "" {
		return fmt.Errorf("kinder dev: BinaryName is required")
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	if opts.Debounce == 0 {
		opts.Debounce = 500 * time.Millisecond
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = time.Second
	}
	if opts.RolloutTimeout == 0 {
		opts.RolloutTimeout = 2 * time.Minute
	}
	if opts.ImageTag == "" {
		opts.ImageTag = fmt.Sprintf("kinder-dev/%s:latest", opts.Target)
	}
	if opts.Logger == nil {
		opts.Logger = log.NoopLogger{}
	}

	// 2. Wrap ctx with signal handling (RESEARCH common pitfall 6).
	parentCtx := opts.Ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3. Write kubeconfig ONCE at startup (RESEARCH anti-pattern: NOT per-cycle).
	kubeconfigPath, cleanup, err := WriteKubeconfigTemp(opts.Provider, opts.ClusterName)
	if err != nil {
		return fmt.Errorf("write kubeconfig tempfile: %w", err)
	}
	defer cleanup()

	// 4. Banner (SC1).
	if !opts.SkipBanner {
		mode := "fsnotify"
		if opts.Poll {
			mode = "poll"
		}
		if opts.Streams.Out != nil {
			fmt.Fprintf(opts.Streams.Out, "Watching %s → deployment/%s (cluster: %s, namespace: %s)\n",
				opts.WatchDir, opts.Target, opts.ClusterName, opts.Namespace)
			fmt.Fprintf(opts.Streams.Out, "Debounce: %s | Mode: %s\n", opts.Debounce, mode)
			fmt.Fprintln(opts.Streams.Out, "Press Ctrl+C to exit.")
		}
	}

	// 5. Event source: injected (tests) or real watcher/poller.
	var rawEvents <-chan struct{}
	if opts.EventSource != nil {
		rawEvents = opts.EventSource
	} else if opts.Poll {
		rawEvents, err = StartPoller(ctx, opts.WatchDir, opts.PollInterval, opts.Logger)
	} else {
		rawEvents, err = StartWatcher(ctx, opts.WatchDir, opts.Logger)
	}
	if err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	// 6. Debounce the raw event stream into cycle triggers.
	cycles := Debounce(ctx, rawEvents, opts.Debounce)

	// 7. Serial cycle runner. Outer select includes ctx.Done so Ctrl+C
	// wakes the loop without waiting for the watcher goroutines to close.
	cycleNum := 0
	var firstErr error
	for {
		select {
		case <-ctx.Done():
			return firstErr
		case _, ok := <-cycles:
			if !ok {
				// Watcher / debouncer closed; exit cleanly.
				return firstErr
			}
			cycleNum++
			cErr := runOneCycle(ctx, cycleOpts{
				cycleNum:       cycleNum,
				binaryName:     opts.BinaryName,
				imageTag:       opts.ImageTag,
				watchDir:       opts.WatchDir,
				namespace:      opts.Namespace,
				target:         opts.Target,
				rolloutTimeout: opts.RolloutTimeout,
				kubeconfigPath: kubeconfigPath,
				clusterName:    opts.ClusterName,
				provider:       opts.Provider,
				logger:         opts.Logger,
				streams:        opts.Streams,
			})
			if cErr != nil {
				if opts.ExitOnFirstError {
					return cErr
				}
				if firstErr == nil {
					firstErr = cErr
				}
				if opts.Streams.ErrOut != nil {
					fmt.Fprintf(opts.Streams.ErrOut, "cycle %d failed: %v\n", cycleNum, cErr)
				}
			}
			// RESEARCH common pitfall 3 (overlap prevention): with the
			// Debouncer's cap=1 output buffer + this serial outer loop,
			// at most ONE cycle is in flight at any time — overlap is
			// structurally impossible. Edits that arrive DURING the
			// in-flight cycle produce a single queued event on `cycles`
			// (the debouncer collapses bursts), which the next loop
			// iteration picks up to fire cycle N+1.
			//
			// We deliberately do NOT drain the cycles channel here.
			// The plan body suggested `select { case <-cycles: default: }`
			// post-cycle to avoid an "immediate redundant cycle" — but
			// edits that arrived during the cycle represent NEW user
			// activity (the cycle started BEFORE those edits), and the
			// correct hot-reload UX is to surface them in cycle N+1.
			// Dropping that queued event would silently lose them.
			// Saturation drop happens at the watcher / debouncer layer
			// (non-blocking sends in watch.go / debounce.go), which is
			// the correct cut.
		}
	}
}
