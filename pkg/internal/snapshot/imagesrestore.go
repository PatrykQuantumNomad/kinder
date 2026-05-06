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

package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// ImportArchiveFn is the function signature used to import a container image
// archive onto all cluster nodes. The default implementation wraps
// nodeutils.LoadImageArchiveWithFallback; tests inject a fake to avoid
// shelling out to containerd.
//
// archivePath is the host-filesystem path to the images.tar file.
type ImportArchiveFn func(ctx context.Context, allNodes []nodes.Node, archivePath string) error

// RestoreImages re-imports images.tar onto every node in allNodes.
//
// When importFn is nil, the production implementation is used, which calls
// nodeutils.LoadImageArchiveWithFallback per node (handles Docker Desktop 27+
// --all-platforms fallback automatically).
//
// Defense-in-depth empty-file gate: if imagesTarHostPath is a 0-byte file,
// RestoreImages returns nil without invoking importFn. The caller (Plan 04)
// also gates on metadata.ImagesDigest == "" before calling RestoreImages; this
// check provides redundant safety at the primitive level.
//
// Error from importFn is wrapped with "restore images:" context and returned.
func RestoreImages(ctx context.Context, allNodes []nodes.Node, imagesTarHostPath string, importFn ImportArchiveFn) error {
	// Defense-in-depth: skip if the file is zero bytes (no images to import).
	info, err := os.Stat(imagesTarHostPath)
	if err != nil {
		return fmt.Errorf("restore images: stat %q: %w", imagesTarHostPath, err)
	}
	if info.Size() == 0 {
		return nil
	}

	if importFn == nil {
		importFn = defaultImportArchive
	}

	if err := importFn(ctx, allNodes, imagesTarHostPath); err != nil {
		return fmt.Errorf("restore images: %w", err)
	}
	return nil
}

// defaultImportArchive calls nodeutils.LoadImageArchiveWithFallback for each
// node in allNodes. It opens the archive file fresh per node (because the tar
// stream is consumed on read and cannot be rewound), matching the API expected
// by LoadImageArchiveWithFallback's openArchive factory parameter.
func defaultImportArchive(ctx context.Context, allNodes []nodes.Node, archivePath string) error {
	for _, n := range allNodes {
		n := n // capture for closure
		openArchive := func() (io.ReadCloser, error) {
			return os.Open(archivePath)
		}
		if err := nodeutils.LoadImageArchiveWithFallback(n, openArchive); err != nil {
			return fmt.Errorf("load image archive on node %q: %w", n.String(), err)
		}
	}
	return nil
}
