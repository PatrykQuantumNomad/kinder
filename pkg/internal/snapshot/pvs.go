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

// pvs.go — capture local-path-provisioner PV data from all cluster nodes.
//
// RESOLUTION TO RESEARCH OPEN QUESTION 8: single tar with per-node
// subdirectories — each node's tar output is a nested entry named
// <nodeName>/local-path-provisioner.tar. This keeps the outer tar entry
// (pvs.tar in the bundle) as a single file, while allowing per-node
// restoration.

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// LocalPathDir is the directory path on each node where local-path-provisioner
// stores PV data. Matches the manifest embedded in:
// pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml
const LocalPathDir = "/opt/local-path-provisioner"

// pvsPathInNode is the (unused in current implementation) in-node path constant.
// We stream tar directly via `tar -cf -` to avoid creating an intermediate file.
const pvsPathInNode = "/tmp/kinder-pvs.tar"

// CapturePVs tars LocalPathDir from each node that has it. The output file at
// dstPath is a host-side tar archive whose entries are named
// "<nodeName>/local-path-provisioner.tar", each containing the verbatim
// per-node `tar -cf - -C /opt local-path-provisioner` stream.
//
// Returns ("", nil) if NO node has LocalPathDir — the caller (Plan 04) should
// treat an empty digest as "no PV data" and write PVsDigest: "" into metadata.
//
// The host tar is teed through sha256 so we can return the digest in one pass.
func CapturePVs(ctx context.Context, allNodes []nodes.Node, dstPath string) (digest string, err error) {
	// Probe each node for the local-path dir.
	type nodeWithData struct {
		node nodes.Node
	}
	var nodesWithData []nodeWithData
	for _, n := range allNodes {
		if probeErr := n.CommandContext(ctx, "test", "-d", LocalPathDir).Run(); probeErr == nil {
			nodesWithData = append(nodesWithData, nodeWithData{node: n})
		}
	}

	f, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("CapturePVs: create dst file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("CapturePVs: close dst file: %w", cerr)
		}
	}()

	if len(nodesWithData) == 0 {
		// No local-path data — write empty file, return empty digest.
		return "", nil
	}

	h := sha256.New()
	mw := io.MultiWriter(f, h)
	tw := tar.NewWriter(mw)

	for _, nwd := range nodesWithData {
		nodeName := nwd.node.String()
		entryName := nodeName + "/local-path-provisioner.tar"

		// Stream `tar -cf - -C /opt local-path-provisioner` from the node.
		// We buffer it so we know the size for the tar header.
		var buf bytes.Buffer
		if err := nwd.node.CommandContext(
			ctx, "tar", "-cf", "-", "-C", "/opt", "local-path-provisioner",
		).SetStdout(&buf).Run(); err != nil {
			return "", fmt.Errorf("CapturePVs: tar from node %s: %w", nodeName, err)
		}

		hdr := &tar.Header{
			Name:     entryName,
			Mode:     0600,
			Size:     int64(buf.Len()),
			Typeflag: tar.TypeReg,
		}
		if whErr := tw.WriteHeader(hdr); whErr != nil {
			return "", fmt.Errorf("CapturePVs: write tar header for %s: %w", entryName, whErr)
		}
		if _, cpErr := io.Copy(tw, &buf); cpErr != nil {
			return "", fmt.Errorf("CapturePVs: write tar entry for %s: %w", entryName, cpErr)
		}
	}

	if err := tw.Close(); err != nil {
		return "", fmt.Errorf("CapturePVs: close tar writer: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
