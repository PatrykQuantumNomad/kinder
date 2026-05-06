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
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
)

// RestorePVs untars per-node PV data from pvsTarHostPath back to
// /opt/local-path-provisioner on each matching node.
//
// The outer tar format (written by Plan 02's CapturePVs) has entries named:
//
//	<nodeName>/local-path-provisioner.tar
//
// For each entry, RestorePVs looks up the node by name and pipes the inner
// tar bytes into `tar -xf - -C /opt` on the matched node.
//
// Nodes not represented in the tar are left untouched (no error, no log).
// Entries whose leading path component does not match any node are skipped
// with a best-effort log line and produce no error.
//
// Empty-file gate: if pvsTarHostPath is a 0-byte file, RestorePVs returns nil
// immediately. This matches the CapturePVs contract (empty PVs → empty file).
//
// Errors across multiple nodes are aggregated via errors.NewAggregate.
func RestorePVs(ctx context.Context, allNodes []nodes.Node, pvsTarHostPath string) error {
	info, err := os.Stat(pvsTarHostPath)
	if err != nil {
		return fmt.Errorf("RestorePVs: stat %q: %w", pvsTarHostPath, err)
	}
	if info.Size() == 0 {
		return nil
	}

	// Build lookup: node name → nodes.Node.
	nodeByName := make(map[string]nodes.Node, len(allNodes))
	for _, n := range allNodes {
		nodeByName[n.String()] = n
	}

	f, err := os.Open(pvsTarHostPath)
	if err != nil {
		return fmt.Errorf("RestorePVs: open outer tar %q: %w", pvsTarHostPath, err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var errs []error

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("RestorePVs: read outer tar header: %w", err)
		}

		// Parse leading path component as node name.
		// Entry name format: "<nodeName>/local-path-provisioner.tar"
		entryName := hdr.Name
		nodeName := strings.SplitN(path.Clean(entryName), "/", 2)[0]

		n, ok := nodeByName[nodeName]
		if !ok {
			// Unknown node — warn and skip (not an error).
			// Log to stderr for observability; caller can ignore.
			fmt.Fprintf(os.Stderr, "RestorePVs: warning: tar entry %q references unknown node %q (skipping)\n",
				entryName, nodeName)
			continue
		}

		// Pipe inner tar bytes into `tar -xf - -C /opt` on the node.
		// Read the entry bytes first so we have the full content for stdin.
		innerBytes, err := io.ReadAll(tr)
		if err != nil {
			errs = append(errs, fmt.Errorf("node %q: read inner tar from outer entry %q: %w", nodeName, entryName, err))
			continue
		}

		cmd := n.CommandContext(ctx,
			"tar", "-xf", "-", "-C", "/opt",
		).SetStdin(newBytesReader(innerBytes))

		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Errorf("node %q: tar -xf failed: %w", nodeName, err))
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

// newBytesReader returns an io.Reader over the given byte slice. Defined as a
// helper to make the call site easier to read.
func newBytesReader(b []byte) io.Reader {
	return strings.NewReader(string(b))
}
