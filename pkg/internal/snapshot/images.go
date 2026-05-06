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

// images.go — capture all containerd images from the bootstrap CP node.
//
// Kind loads identical images to all nodes for the k8s.io namespace, so
// capturing from the bootstrap CP is sufficient (per RESEARCH §2).
//
// Pattern mirrors pkg/cluster/nodeutils/util.go LoadImageArchive: export via
// ctr, then cat the file back to the host via node.Command("cat", path).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

// imagesPathInNode is the path inside the CP node container where
// `ctr images export` writes the archive before we stream it out.
const imagesPathInNode = "/tmp/kinder-images.tar"

// ListImageRefs returns the list of image references in the k8s.io containerd
// namespace on the CP node. It uses `ctr --namespace=k8s.io images list -q`
// which outputs one ref per line.
func ListImageRefs(ctx context.Context, cp nodes.Node) ([]string, error) {
	lines, err := exec.OutputLines(cp.CommandContext(ctx,
		"ctr", "--namespace=k8s.io", "images", "list", "-q",
	))
	if err != nil {
		return nil, fmt.Errorf("ListImageRefs: ctr images list: %w", err)
	}
	var refs []string
	for _, line := range lines {
		if r := strings.TrimSpace(line); r != "" {
			refs = append(refs, r)
		}
	}
	return refs, nil
}

// CaptureImages exports all containerd images from the k8s.io namespace on the
// bootstrap CP node to dstPath on the host and returns the sha256 hex digest.
//
// Steps:
//  1. List refs via ListImageRefs.
//  2. If empty → write zero-byte file; return sha256("").
//  3. ctr --namespace=k8s.io images export <pathInNode> <refs...>
//  4. cat <pathInNode> teed through sha256 to dstPath.
//  5. Best-effort cleanup: rm -f <pathInNode>.
func CaptureImages(ctx context.Context, cp nodes.Node, dstPath string) (digest string, err error) {
	refs, err := ListImageRefs(ctx, cp)
	if err != nil {
		return "", fmt.Errorf("CaptureImages: list image refs: %w", err)
	}

	f, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("CaptureImages: create dst file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("CaptureImages: close dst file: %w", cerr)
		}
	}()

	h := sha256.New()

	if len(refs) == 0 {
		// No images — write empty file, digest of empty bytes.
		return hex.EncodeToString(h.Sum(nil)), nil
	}

	// Export all images to a tar inside the node.
	exportArgs := append(
		[]string{"--namespace=k8s.io", "images", "export", imagesPathInNode},
		refs...,
	)
	if err := cp.CommandContext(ctx, "ctr", exportArgs...).Run(); err != nil {
		return "", fmt.Errorf("CaptureImages: ctr images export: %w", err)
	}

	// Stream the exported archive back to the host, teed through sha256.
	mw := io.MultiWriter(f, h)
	if err := cp.CommandContext(ctx, "cat", imagesPathInNode).SetStdout(mw).Run(); err != nil {
		return "", fmt.Errorf("CaptureImages: stream images archive from node: %w", err)
	}

	// Best-effort cleanup.
	_ = cp.CommandContext(ctx, "rm", "-f", imagesPathInNode).Run()

	return hex.EncodeToString(h.Sum(nil)), nil
}
