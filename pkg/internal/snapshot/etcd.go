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

// etcd.go — etcd snapshot capture from a running kinder cluster node.
//
// Implementation note: etcdctlAuthArgs is duplicated inline from
// pkg/internal/doctor/resumereadiness.go. We do NOT import the doctor package
// to avoid an import cycle (doctor imports snapshot-adjacent packages).
// TODO: refactor to a shared internal constant if the cert paths ever move.
// See: sigs.k8s.io/kind/pkg/internal/doctor.etcdctlAuthArgs

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

// etcdSnapshotPathInNode is the path inside the node container where etcdctl
// writes the snapshot before we stream it to the host.
const etcdSnapshotPathInNode = "/tmp/kinder-etcd.snap"

// etcdctlAuthArgs is the cert/endpoint argument tuple for etcdctl invocations.
// Duplicated from pkg/internal/doctor/resumereadiness.go — see file-level note.
var etcdctlAuthArgs = []string{
	"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
	"--cert=/etc/kubernetes/pki/etcd/peer.crt",
	"--key=/etc/kubernetes/pki/etcd/peer.key",
	"--endpoints=https://127.0.0.1:2379",
}

// CaptureEtcd takes an etcd snapshot via `crictl exec` into the running etcd
// static-pod container, streams the resulting file out to dstPath on the host,
// and returns the sha256 hex digest of the written file.
//
// Steps:
//  1. Discover the etcd container id via `crictl ps --name etcd -q`.
//  2. Run `crictl exec <id> etcdctl <authArgs> snapshot save <pathInNode>`.
//  3. Stream the file out via `cat <pathInNode>` teed through sha256.
//  4. Best-effort cleanup: `rm -f <pathInNode>`.
func CaptureEtcd(ctx context.Context, cp nodes.Node, dstPath string) (digest string, err error) {
	// 1. Discover etcd container id.
	idLines, err := exec.OutputLines(cp.CommandContext(ctx, "crictl", "ps", "--name", "etcd", "-q"))
	if err != nil {
		return "", fmt.Errorf("CaptureEtcd: crictl ps for etcd container: %w", err)
	}
	var containerID string
	for _, line := range idLines {
		if id := strings.TrimSpace(line); id != "" {
			containerID = id
			break
		}
	}
	if containerID == "" {
		return "", fmt.Errorf("CaptureEtcd: etcd container not running on node %s (cluster may not be running)", cp.String())
	}

	// 2. Run etcdctl snapshot save inside the etcd container.
	saveArgs := append(
		[]string{"exec", containerID, "etcdctl"},
		append(etcdctlAuthArgs, "snapshot", "save", etcdSnapshotPathInNode)...,
	)
	if err := cp.CommandContext(ctx, "crictl", saveArgs...).Run(); err != nil {
		return "", fmt.Errorf("CaptureEtcd: etcdctl snapshot save: %w", err)
	}

	// 3. Open destination file and stream via cat, teed through sha256.
	f, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("CaptureEtcd: create dst file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("CaptureEtcd: close dst file: %w", cerr)
		}
	}()

	h := sha256.New()
	mw := io.MultiWriter(f, h)
	if err := cp.CommandContext(ctx, "cat", etcdSnapshotPathInNode).SetStdout(mw).Run(); err != nil {
		return "", fmt.Errorf("CaptureEtcd: stream snapshot from node: %w", err)
	}

	// 4. Best-effort cleanup.
	_ = cp.CommandContext(ctx, "rm", "-f", etcdSnapshotPathInNode).Run()

	return hex.EncodeToString(h.Sum(nil)), nil
}
