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
	"io"
	"time"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

// cycleOpts is the unexported per-cycle configuration runOneCycle consumes.
// Fields are bundled from Options so unit tests can construct cycleOpts
// directly without the full Run scaffolding.
type cycleOpts struct {
	cycleNum       int
	binaryName     string
	imageTag       string
	watchDir       string
	namespace      string
	target         string
	rolloutTimeout time.Duration
	kubeconfigPath string
	clusterName    string
	provider       *cluster.Provider
	logger         log.Logger
	streams        cmd.IOStreams
}

// loadImagesFn is the package-level test-injection point that runOneCycle
// uses to invoke the image-load step. Production routes through
// LoadImagesIntoCluster (which wires Provider/Logger/etc.); tests swap this
// to a no-op or error fake without building a fake *cluster.Provider.
//
// Why a separate var (vs. injecting via LoadOptions.ImageLoaderFn): the
// upstream LoadOptions.ImageLoaderFn only fakes the per-node loader — the
// preceding pipeline (imageInspectID + nodeLister + imageTagsFn) still
// needs scaffolding. cycle.go's tests are about cycle ORCHESTRATION, not
// load internals (which are exhaustively covered by load_test.go), so a
// single high-level injection point is the cleaner cut.
var loadImagesFn = LoadImagesIntoCluster

// runOneCycle executes one build → load → rollout cycle and prints
// per-step timing to opts.streams.Out. On any step failure, returns the
// wrapped error WITHOUT running subsequent steps.
//
// Output format (RESEARCH §7, matching Phase 47/48 %.1fs convention):
//
//	[cycle N] Change detected — starting build-load-rollout
//	  build:   X.Xs
//	  load:    X.Xs
//	  rollout: X.Xs
//	  total:   X.Xs
//
// On failure the line for the failed step still prints with the partial
// elapsed time and a follow-up "(error: <msg>)" annotation. Subsequent step
// lines are NOT printed. The "total:" line is omitted on failure — users
// reading the output already see the failed step's elapsed time and know
// the cycle aborted.
//
// runOneCycle defends against streams.Out == nil (substituting io.Discard)
// so a partially-initialized cycleOpts from a buggy CLI shell does not
// panic on the first Fprintf.
func runOneCycle(ctx context.Context, opts cycleOpts) error {
	out := opts.streams.Out
	if out == nil {
		out = io.Discard
	}

	cycleStart := time.Now()
	fmt.Fprintf(out, "[cycle %d] Change detected — starting build-load-rollout\n", opts.cycleNum)

	// 1. build
	t0 := time.Now()
	if err := BuildImageFn(ctx, opts.binaryName, opts.imageTag, opts.watchDir); err != nil {
		fmt.Fprintf(out, "  build:   %.1fs (error: %v)\n", time.Since(t0).Seconds(), err)
		return fmt.Errorf("build: %w", err)
	}
	fmt.Fprintf(out, "  build:   %.1fs\n", time.Since(t0).Seconds())

	// 2. load
	t1 := time.Now()
	loadErr := loadImagesFn(ctx, LoadOptions{
		ClusterName: opts.clusterName,
		ImageTag:    opts.imageTag,
		BinaryName:  opts.binaryName,
		Logger:      opts.logger,
		Provider:    opts.provider,
	})
	if loadErr != nil {
		fmt.Fprintf(out, "  load:    %.1fs (error: %v)\n", time.Since(t1).Seconds(), loadErr)
		return fmt.Errorf("load: %w", loadErr)
	}
	fmt.Fprintf(out, "  load:    %.1fs\n", time.Since(t1).Seconds())

	// 3. rollout
	t2 := time.Now()
	if err := RolloutFn(ctx, opts.kubeconfigPath, opts.namespace, opts.target, opts.rolloutTimeout); err != nil {
		fmt.Fprintf(out, "  rollout: %.1fs (error: %v)\n", time.Since(t2).Seconds(), err)
		return fmt.Errorf("rollout: %w", err)
	}
	fmt.Fprintf(out, "  rollout: %.1fs\n", time.Since(t2).Seconds())

	// 4. total
	fmt.Fprintf(out, "  total:   %.1fs\n", time.Since(cycleStart).Seconds())
	return nil
}
