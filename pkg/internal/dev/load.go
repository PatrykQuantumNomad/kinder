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
	"os"
	"path/filepath"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	kerrors "sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/fs"
	"sigs.k8s.io/kind/pkg/log"
)

// ImageLoaderFn is the per-node image loader signature, matching the
// signature of nodeutils.LoadImageArchiveWithFallback. Tests inject a fake
// to capture which nodes received load attempts without spinning a real
// cluster.
type ImageLoaderFn func(node nodes.Node, openTar func() (io.ReadCloser, error)) error

// LoadOptions carries inputs to LoadImagesIntoCluster. Each field is
// explicit so the cycle runner (Plan 03) can build a fresh options struct
// per cycle without sharing state.
type LoadOptions struct {
	ClusterName string            // for nodeLister(clusterName)
	ImageTag    string            // image to load (single image per cycle)
	BinaryName  string            // docker | podman | nerdctl | ...
	Logger      log.Logger        // for retag-failure warnings
	Provider    *cluster.Provider // production: ListInternalNodes target
	// ImageLoaderFn is the per-node loader. nil → use the production
	// nodeutils.LoadImageArchiveWithFallback. Tests pass a fake.
	ImageLoaderFn ImageLoaderFn
}

// nodeLister is the indirection point for provider.ListInternalNodes. In
// production it forwards to the Provider stored on LoadOptions. Tests swap
// it via withNodeLister to avoid building a fake *cluster.Provider.
//
// The function is set per-call from opts.Provider during LoadImagesIntoCluster
// — this lets tests override it without overriding opts.Provider (which is
// awkward to fake).
var nodeLister = func(clusterName string) ([]nodes.Node, error) {
	// This default closure is replaced inside LoadImagesIntoCluster when
	// opts.Provider is non-nil. If a test calls into here without first
	// swapping nodeLister, that's a bug — return a clear error.
	return nil, fmt.Errorf("nodeLister not configured (test must swap before calling LoadImagesIntoCluster, or LoadOptions.Provider must be set)")
}

// imageTagsFn is the indirection for nodeutils.ImageTags. Tests swap this
// to script per-node behavior without driving real crictl JSON through a
// FakeNode.
var imageTagsFn = nodeutils.ImageTags

// reTagFn is the indirection for nodeutils.ReTagImage.
var reTagFn = nodeutils.ReTagImage

// imageInspectID runs `<binary> image inspect -f {{ .Id }} <ref>` on the
// host to look up the local image ID. Wrapped as a package-level var for
// test injection.
var imageInspectID = func(ctx context.Context, binaryName, ref string) (string, error) {
	cmd := devCmder.CommandContext(ctx, binaryName, "image", "inspect", "-f", "{{ .Id }}", ref)
	// Local copy of exec.OutputLines logic to avoid double-import; the
	// Cmd interface is the same.
	pr, pw := io.Pipe()
	cmd.SetStdout(pw)
	type res struct {
		lines []string
		err   error
	}
	out := make(chan res, 1)
	go func() {
		buf, readErr := io.ReadAll(pr)
		_ = pr.Close()
		var lines []string
		if len(buf) > 0 {
			// Split on \n; the last line may be empty if the output ends
			// with a newline.
			start := 0
			for i, b := range buf {
				if b == '\n' {
					lines = append(lines, string(buf[start:i]))
					start = i + 1
				}
			}
			if start < len(buf) {
				lines = append(lines, string(buf[start:]))
			}
		}
		out <- res{lines: lines, err: readErr}
	}()
	runErr := cmd.Run()
	_ = pw.Close()
	r := <-out
	if runErr != nil {
		return "", runErr
	}
	if r.err != nil {
		return "", r.err
	}
	if len(r.lines) == 0 {
		return "", fmt.Errorf("no image ID returned for %q", ref)
	}
	return r.lines[0], nil
}

// LoadImagesIntoCluster imports the named image into every cluster node,
// replicating the core pipeline of `kinder load images` against public
// APIs (RESEARCH §1 Option A — extracts the logic without calling the
// unexported runE in pkg/cmd/kind/load/images/images.go).
//
// Pipeline (matches the upstream behavior as of phase 49 research):
//  1. imageInspectID → host image ID for opts.ImageTag.
//  2. nodeLister(opts.ClusterName) → all internal cluster nodes.
//  3. For each node: imageTagsFn → if ID present with right tag, skip;
//     if present with different tag, reTagFn in place (fallback to load
//     on failure); if absent, mark as candidate.
//  4. If no candidates → return nil (smart-skip path).
//  5. fs.TempDir + `<binary> save -o <tar> <imageTag>` → write image to
//     host temp file.
//  6. errors.UntilErrorConcurrent: per-candidate-node, call
//     opts.ImageLoaderFn (default nodeutils.LoadImageArchiveWithFallback)
//     with a tar opener that re-opens for each retry.
//
// Why we don't reuse the unexported runE in load/images/images.go: it is
// unexported, takes an unexported flagpole, and writes structured output
// to streams.Out. RESEARCH §1 Option A is the chosen path.
//
// Why a single-image API rather than multi-image (`runE` accepts []string):
// `kinder dev` only ever loads ONE image per cycle (the freshly-built one).
// Multi-image surface adds complexity for no gain.
func LoadImagesIntoCluster(ctx context.Context, opts LoadOptions) error {
	if opts.ImageTag == "" {
		return fmt.Errorf("dev load: ImageTag empty")
	}
	if opts.ClusterName == "" {
		return fmt.Errorf("dev load: ClusterName empty")
	}
	if opts.BinaryName == "" {
		return fmt.Errorf("dev load: BinaryName empty")
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.NoopLogger{}
	}

	loader := opts.ImageLoaderFn
	if loader == nil {
		loader = nodeutils.LoadImageArchiveWithFallback
	}

	// 1. Inspect image ID locally.
	imageID, err := imageInspectID(ctx, opts.BinaryName, opts.ImageTag)
	if err != nil {
		return fmt.Errorf("inspect image %q: %w", opts.ImageTag, err)
	}

	// 2. List internal nodes. Use the Provider when set; tests swap
	// nodeLister directly so they can avoid building a fake Provider.
	lister := nodeLister
	if opts.Provider != nil {
		lister = opts.Provider.ListInternalNodes
	}
	internalNodes, err := lister(opts.ClusterName)
	if err != nil {
		return fmt.Errorf("list nodes for cluster %q: %w", opts.ClusterName, err)
	}

	// 3. Smart-skip + candidate selection.
	var candidates []nodes.Node
	for _, n := range internalNodes {
		tags, tagsErr := imageTagsFn(n, imageID)
		if tagsErr != nil {
			// Node not reachable / crictl unavailable / no such image →
			// defensively treat as a candidate so we attempt to load.
			candidates = append(candidates, n)
			continue
		}
		if _, hasTag := tags[opts.ImageTag]; hasTag {
			continue // already has correct tag
		}
		if len(tags) > 0 {
			// Image present under a different tag — re-tag in place
			// (fast path; avoids re-import).
			if rtErr := reTagFn(n, imageID, opts.ImageTag); rtErr != nil {
				logger.Warnf("retag on node %s failed, will reimport: %v", n.String(), rtErr)
				candidates = append(candidates, n)
			}
			continue
		}
		// Image absent.
		candidates = append(candidates, n)
	}
	if len(candidates) == 0 {
		return nil
	}

	// 4. Save the image to a host temp tar.
	tempDir, err := fs.TempDir("", "kinder-dev-load-")
	if err != nil {
		return fmt.Errorf("create temp dir for image tar: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	tarPath := filepath.Join(tempDir, "image.tar")

	saveCmd := devCmder.CommandContext(ctx, opts.BinaryName, "save", "-o", tarPath, opts.ImageTag)
	if saveErr := saveCmd.Run(); saveErr != nil {
		return fmt.Errorf("save image %q to tar: %w", opts.ImageTag, saveErr)
	}

	// 5. Concurrent per-candidate-node load via UntilErrorConcurrent.
	fns := make([]func() error, 0, len(candidates))
	for _, n := range candidates {
		n := n
		fns = append(fns, func() error {
			return loader(n, func() (io.ReadCloser, error) {
				return os.Open(tarPath)
			})
		})
	}
	return kerrors.UntilErrorConcurrent(fns)
}
