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

// Package dev implements the `kinder dev --watch <dir> --target <deployment>`
// command. It enters a watch-mode loop that triggers a build → load →
// rollout cycle whenever a file in the watched directory changes.
//
// The actual orchestration lives in pkg/internal/dev (Plan 49-03's Run +
// Options). This file is a thin Cobra wrapper: parse flags, validate input,
// resolve cluster name + provider binary, build internaldev.Options, and call
// internaldev.Run.
package dev

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/cli"
	internaldev "sigs.k8s.io/kind/pkg/internal/dev"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/internal/runtime"
	"sigs.k8s.io/kind/pkg/log"
)

// flagpole holds the parsed flag values for `kinder dev`.
type flagpole struct {
	WatchDir       string        // --watch (required)
	Target         string        // --target (required)
	ClusterName    string        // --name; "" = auto-detect via lifecycle.ResolveClusterName
	Image          string        // --image; "" = Run derives "kinder-dev/<target>:latest"
	Namespace      string        // --namespace
	Debounce       time.Duration // --debounce
	Poll           bool          // --poll
	PollInterval   time.Duration // --poll-interval
	RolloutTimeout time.Duration // --rollout-timeout
	JSON           bool          // --json (reserved; current Run does not emit JSON)
}

// runFn is the test-injection point for internaldev.Run. Production code calls
// internaldev.Run; tests substitute a fake to capture Options without spinning
// a real watcher or cluster (matches pause.go's pauseFn pattern).
var runFn = internaldev.Run

// resolveClusterName is the test-injection point for cluster name resolution.
// Production wires it to lifecycle.ResolveClusterName backed by a real
// *cluster.Provider (matches pause.go:56). The explicit argument is the value
// of --name; when empty, ResolveClusterName auto-detects when there is exactly
// one cluster.
var resolveClusterName = func(args []string, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(log.NoopLogger{}),
		runtime.GetDefault(log.NoopLogger{}),
	)
	return lifecycle.ResolveClusterName(args, provider)
}

// resolveBinaryName is the test-injection point for provider binary
// resolution. Production calls lifecycle.ProviderBinaryName which returns the
// first available container runtime in $PATH ("docker", "podman", "nerdctl"),
// or "" if none are present (RESEARCH §1 / §9 — single source of truth, no
// manual binary detection in this package).
var resolveBinaryName = lifecycle.ProviderBinaryName

// NewCommand returns a new cobra.Command for `kinder dev`.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Use:   "dev",
		Short: "Watch a directory and run build → load → rollout cycles on file change",
		Long: "kinder dev enters a watch-mode loop. On every file save in --watch, it builds a Docker image,\n" +
			"imports it via the kinder load images pipeline, and rolls the target Deployment via\n" +
			"kubectl rollout restart. Cycle timings are printed per cycle.\n\n" +
			"PREREQUISITES:\n" +
			"  - The target Deployment must already reference the image tag (default: kinder-dev/<target>:latest).\n" +
			"  - The Deployment must use imagePullPolicy: Never so containerd uses the locally-loaded image.\n" +
			"  - kubectl and docker (or the active provider binary) must be on PATH; kinder doctor verifies both.\n\n" +
			"On Docker Desktop for macOS where fsnotify events are unreliable on volume-mounted directories,\n" +
			"pass --poll to switch to a polling-based watcher.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(cmd, logger, streams, flags, args)
		},
	}

	c.Flags().StringVar(&flags.WatchDir, "watch", "",
		"directory to watch for file changes (REQUIRED)")
	c.Flags().StringVar(&flags.Target, "target", "",
		"target Deployment name to roll on each cycle (REQUIRED)")
	c.Flags().StringVarP(&flags.ClusterName, "name", "n", "",
		"cluster name (default: auto-detect when there is exactly one cluster)")
	c.Flags().StringVar(&flags.Image, "image", "",
		"image tag to build/load (default: kinder-dev/<target>:latest)")
	c.Flags().StringVar(&flags.Namespace, "namespace", "default",
		"Kubernetes namespace of the target Deployment")
	c.Flags().DurationVar(&flags.Debounce, "debounce", 500*time.Millisecond,
		"debounce window for file events (e.g. 500ms, 1s)")
	c.Flags().BoolVar(&flags.Poll, "poll", false,
		"use stdlib polling instead of fsnotify (recommended on Docker Desktop macOS volume mounts)")
	c.Flags().DurationVar(&flags.PollInterval, "poll-interval", time.Second,
		"polling interval when --poll is set (e.g. 1s, 500ms)")
	c.Flags().DurationVar(&flags.RolloutTimeout, "rollout-timeout", 2*time.Minute,
		"kubectl rollout status timeout per cycle (e.g. 2m, 90s)")
	c.Flags().BoolVar(&flags.JSON, "json", false,
		"reserved; currently unused (per-cycle output is human-readable)")

	_ = c.MarkFlagRequired("watch")
	_ = c.MarkFlagRequired("target")
	return c
}

// runE executes the parsed `kinder dev` command. It validates --watch points
// to an existing directory, validates duration flags, resolves the cluster
// name (auto-detecting when --name is unset), resolves the container runtime
// binary, builds the internaldev.Options, and delegates to runFn.
func runE(cobraCmd *cobra.Command, logger log.Logger, streams cmd.IOStreams, flags *flagpole, args []string) error {
	// Validate --watch (must exist + be a directory). Errors here MUST fire
	// before we resolve cluster name or build a provider so we fail fast.
	info, err := os.Stat(flags.WatchDir)
	if err != nil {
		return fmt.Errorf("invalid --watch %q: %w", flags.WatchDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("invalid --watch %q: not a directory", flags.WatchDir)
	}

	// Validate durations. cobra.DurationVar already rejects bare integers
	// (Phase 47-06 lesson); these guards catch negative / zero values that
	// parse cleanly but are nonsensical for a watch loop.
	if flags.Debounce < 0 {
		return fmt.Errorf("invalid --debounce %v: must be >= 0", flags.Debounce)
	}
	if flags.PollInterval <= 0 {
		return fmt.Errorf("invalid --poll-interval %v: must be > 0", flags.PollInterval)
	}
	if flags.RolloutTimeout <= 0 {
		return fmt.Errorf("invalid --rollout-timeout %v: must be > 0", flags.RolloutTimeout)
	}

	// Resolve cluster name. When --name is unset we auto-detect (matches
	// the Phase 47-06 / 48-05 convention used by pause/resume/snapshot).
	name, err := resolveClusterName(args, flags.ClusterName)
	if err != nil {
		return err
	}

	// Resolve container runtime binary. ProviderBinaryName returns "" when
	// no docker/podman/nerdctl is on PATH; surface a clear error rather
	// than letting Run discover the empty string later.
	binary := resolveBinaryName()
	if binary == "" {
		return fmt.Errorf("no container runtime binary found in PATH (looked for docker, podman, nerdctl); install one or run `kinder doctor`")
	}

	// Build a provider for Run. Production lifecycle helpers use the same
	// cluster.NewProvider(...) pattern; tests stub runFn entirely so the
	// provider is never dereferenced in tests.
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	opts := internaldev.Options{
		Ctx:            cobraCmd.Context(),
		ClusterName:    name,
		Provider:       provider,
		BinaryName:     binary,
		WatchDir:       flags.WatchDir,
		Target:         flags.Target,
		Namespace:      flags.Namespace,
		ImageTag:       flags.Image, // empty → Run derives kinder-dev/<target>:latest
		Debounce:       flags.Debounce,
		Poll:           flags.Poll,
		PollInterval:   flags.PollInterval,
		RolloutTimeout: flags.RolloutTimeout,
		Logger:         logger,
		Streams:        streams,
	}
	return runFn(opts)
}
