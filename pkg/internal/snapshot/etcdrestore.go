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

// Package snapshot implements kinder snapshot capture and restore primitives.
package snapshot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

// EtcdRestoreOptions controls a single RestoreEtcd invocation.
//
// Preconditions (enforced by the caller, Plan 04):
//   - The cluster is paused (all node containers are running but kubelet has
//     been given time to settle; etcd containers will be displaced by manifest removal).
//   - CP containers are already started (Plan 04 resumes CP-only first).
//
// RestoreEtcd is deliberately stateless: all cluster-topology knowledge comes
// from CPs and ProviderBin. It does NOT call lifecycle.Pause / lifecycle.Resume
// to avoid circular imports (lifecycle is an upstream caller of this package).
type EtcdRestoreOptions struct {
	// CPs is the ordered list of control-plane nodes. For single-CP, len == 1.
	// For HA, the SAME snapshot and a fresh cluster token are applied to all.
	CPs []nodes.Node

	// SnapshotHostPath is the host filesystem path to the etcd.snap file
	// previously extracted from the bundle (by Plan 04's orchestrator).
	SnapshotHostPath string

	// Token overrides the default cluster token. When empty, RestoreEtcd
	// generates a fresh token: "kind-snap-restore-<unix-nanoseconds>" to
	// satisfy RESEARCH Pitfall 5 — avoids cluster-id collision when the
	// SAME snapshot is restored onto an existing peer set.
	Token string

	// ProviderBin is the container runtime binary ("docker", "podman", "nerdctl")
	// used for IP discovery via `<ProviderBin> inspect`.
	ProviderBin string
}

// etcdManifestPath is the static-pod manifest path inside kindest/node.
const etcdManifestPath = "/etc/kubernetes/manifests/etcd.yaml"

// etcdManifestAside is the temporary location used while the restore is in
// progress. Moving the manifest aside causes kubelet to stop managing the etcd
// static pod, giving us a quiescent etcd data dir to operate on.
const etcdManifestAside = "/tmp/etcd-manifest.yaml.kinder-snap"

// etcdDataDir is the production etcd data directory inside kindest/node.
const etcdDataDir = "/var/lib/etcd"

// etcdRestoredDir is the temporary destination for etcdctl snapshot restore.
// We restore here first, then atomically swap with etcdDataDir.
const etcdRestoredDir = "/var/lib/etcd-restored"

// etcdOldDataDir is the backup location during the atomic swap. On success it
// is left in place (cleanup is not critical for a restore operation). On failure
// it is used as the rollback source.
const etcdOldDataDir = "/var/lib/etcd.kinder-old"

// etcdTmpSnap is the path to which the host snapshot file is copied inside the
// node before handing it to etcdctl.
const etcdTmpSnap = "/tmp/kinder-restore.snap"

// etcdManifestSettleDelay is the duration RestoreEtcd waits after moving the
// etcd manifest aside so kubelet has time to tear down the etcd static pod.
// Tests replace this variable to avoid sleeping.
var etcdManifestSettleDelay = 5 * time.Second

// RestoreEtcd restores the etcd snapshot at opts.SnapshotHostPath onto each
// control-plane node in opts.CPs.
//
// For HA clusters the restore runs in parallel across all CP nodes, all using
// the same snapshot data and the same fresh --initial-cluster-token (required
// to avoid cluster-id collision, RESEARCH Pitfall 5). The --initial-cluster
// membership string is also identical across all CPs so etcd bootstraps with
// a consistent peer list.
//
// Per-node steps:
//  1. Move etcd manifest aside → kubelet stops etcd.
//  2. Wait briefly for kubelet to tear down the etcd container.
//  3. Copy host snapshot into the node.
//  4. Run etcdctl snapshot restore with per-node advertise-peer-urls.
//  5. Atomically swap data dirs; roll back on failure.
//  6. Restore manifest → kubelet starts etcd from the restored data dir.
//  7. Remove tmp snapshot copy.
//
// Errors are aggregated. A failing CP node does NOT abort other CP nodes;
// callers should treat a partial failure as an inconsistent cluster state
// (per the "atomicity = pre-flight + fail-fast" CONTEXT.md decision).
func RestoreEtcd(ctx context.Context, opts EtcdRestoreOptions) error {
	if len(opts.CPs) == 0 {
		return errors.New("RestoreEtcd: no control-plane nodes provided")
	}
	if opts.SnapshotHostPath == "" {
		return errors.New("RestoreEtcd: SnapshotHostPath is empty")
	}

	// Generate the cluster token once — all CPs in an HA cluster must share
	// the same value so etcd members recognise each other post-restore.
	token := opts.Token
	if token == "" {
		token = fmt.Sprintf("kind-snap-restore-%d", time.Now().UnixNano())
	}

	// Build the --initial-cluster membership string.  For single-CP we use
	// the well-known loopback address; for HA we query each node's container IP.
	initialCluster, nodeIPs, err := buildInitialCluster(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "RestoreEtcd: failed to build initial-cluster string")
	}

	// Read the snapshot bytes once; every node gets the same bytes via stdin.
	snapBytes, err := os.ReadFile(opts.SnapshotHostPath)
	if err != nil {
		return errors.Wrapf(err, "RestoreEtcd: failed to read snapshot file %q", opts.SnapshotHostPath)
	}

	// Run per-node restore in parallel (each node touches only its own data dir).
	fns := make([]func() error, len(opts.CPs))
	for i, cp := range opts.CPs {
		cp := cp // capture
		ip := nodeIPs[i]
		fns[i] = func() error {
			return restoreSingleCP(ctx, cp, ip, initialCluster, token, snapBytes)
		}
	}

	return errors.AggregateConcurrent(fns)
}

// buildInitialCluster returns:
//   - the comma-separated --initial-cluster string
//   - a slice of IPs (one per CP, in the same order as opts.CPs)
//
// For single-CP, the IP is always "127.0.0.1".
// For HA, the IP is fetched via `<ProviderBin> inspect --format {{.NetworkSettings.Networks.kind.IPAddress}}`.
func buildInitialCluster(ctx context.Context, opts EtcdRestoreOptions) (string, []string, error) {
	ips := make([]string, len(opts.CPs))
	members := make([]string, len(opts.CPs))

	for i, cp := range opts.CPs {
		var ip string
		if len(opts.CPs) == 1 {
			ip = "127.0.0.1"
		} else {
			var err error
			ip, err = containerIP(ctx, cp, opts.ProviderBin)
			if err != nil {
				return "", nil, errors.Wrapf(err, "failed to get IP for node %q", cp.String())
			}
		}
		ips[i] = ip
		members[i] = fmt.Sprintf("%s=https://%s:2380", cp.String(), ip)
	}

	return strings.Join(members, ","), ips, nil
}

// containerIP returns the kind-network IPv4 address of a node container by
// running `<providerBin> inspect --format {{.NetworkSettings.Networks.kind.IPAddress}} <node>`.
func containerIP(ctx context.Context, n nodes.Node, providerBin string) (string, error) {
	var buf bytes.Buffer
	cmd := n.CommandContext(ctx,
		providerBin, "inspect",
		"--format", "{{.NetworkSettings.Networks.kind.IPAddress}}",
		n.String(),
	).SetStdout(&buf)
	if err := cmd.Run(); err != nil {
		return "", errors.Wrapf(err, "inspect node %q for IP", n.String())
	}
	ip := strings.TrimSpace(buf.String())
	if ip == "" {
		return "", errors.Errorf("inspect returned empty IP for node %q", n.String())
	}
	return ip, nil
}

// restoreSingleCP carries out the full restore sequence on one CP node.
// All errors cause an early return (fail-fast); manifest rollback is attempted
// whenever the manifest was successfully moved aside.
func restoreSingleCP(
	ctx context.Context,
	cp nodes.Node,
	ip string,
	initialCluster string,
	token string,
	snapBytes []byte,
) (retErr error) {
	nodeName := cp.String()

	// Step 1: move etcd static-pod manifest aside so kubelet stops etcd.
	if err := run(cp.CommandContext(ctx, "mv", etcdManifestPath, etcdManifestAside)); err != nil {
		return errors.Wrapf(err, "node %q: move etcd manifest aside", nodeName)
	}

	// Deferred manifest restore: ensures manifest is always put back, even on
	// error, so kubelet can eventually restart etcd.  The swap steps (5) and
	// data-dir rollback are handled inline below.
	defer func() {
		if mvErr := run(cp.CommandContext(ctx, "mv", etcdManifestAside, etcdManifestPath)); mvErr != nil {
			// Wrap into the existing error (if any) so the caller sees both.
			if retErr != nil {
				retErr = errors.NewAggregate([]error{retErr, errors.Wrapf(mvErr, "node %q: restore etcd manifest after failure", nodeName)})
			} else {
				retErr = errors.Wrapf(mvErr, "node %q: restore etcd manifest", nodeName)
			}
		}
	}()

	// Step 2: wait for kubelet to tear down the etcd container.
	time.Sleep(etcdManifestSettleDelay)

	// Step 3: copy host snapshot bytes into the node via stdin.
	if err := run(
		cp.CommandContext(ctx, "cp", "/dev/stdin", etcdTmpSnap).
			SetStdin(bytes.NewReader(snapBytes)),
	); err != nil {
		return errors.Wrapf(err, "node %q: copy snapshot into node", nodeName)
	}

	// Step 4: run etcdctl snapshot restore.
	if err := etcdctlRestore(ctx, cp, nodeName, ip, initialCluster, token); err != nil {
		// Cleanup tmp snap (best-effort).
		_ = run(cp.CommandContext(ctx, "rm", "-f", etcdTmpSnap))
		return errors.Wrapf(err, "node %q: etcdctl snapshot restore", nodeName)
	}

	// Step 5: atomic data-dir swap.
	// 5a: mv /var/lib/etcd → /var/lib/etcd.kinder-old
	if err := run(cp.CommandContext(ctx, "mv", etcdDataDir, etcdOldDataDir)); err != nil {
		_ = run(cp.CommandContext(ctx, "rm", "-f", etcdTmpSnap))
		return errors.Wrapf(err, "node %q: rename old etcd data dir", nodeName)
	}

	// 5b: mv /var/lib/etcd-restored → /var/lib/etcd
	if err := run(cp.CommandContext(ctx, "mv", etcdRestoredDir, etcdDataDir)); err != nil {
		// Rollback: try to restore old data dir.
		_ = run(cp.CommandContext(ctx, "mv", etcdOldDataDir, etcdDataDir))
		_ = run(cp.CommandContext(ctx, "rm", "-f", etcdTmpSnap))
		return errors.Wrapf(err, "node %q: swap restored etcd data dir", nodeName)
	}

	// Step 7: cleanup tmp snapshot copy (manifest is restored by the deferred call above).
	_ = run(cp.CommandContext(ctx, "rm", "-f", etcdTmpSnap))

	return nil
}

// etcdctlRestore runs `etcdctl snapshot restore` inside the node.
// etcdctl is available on the kindest/node PATH (RESEARCH Open Question 6
// confirmed: it ships in the node image for all supported K8s versions via
// the etcd static-pod container's bind-mount or the node image itself).
// If etcdctl is absent the command will fail and the caller surfaces the error.
func etcdctlRestore(
	ctx context.Context,
	cp nodes.Node,
	nodeName string,
	ip string,
	initialCluster string,
	token string,
) error {
	advertise := fmt.Sprintf("https://%s:2380", ip)
	return run(cp.CommandContext(ctx,
		"etcdctl", "snapshot", "restore", etcdTmpSnap,
		"--data-dir="+etcdRestoredDir,
		"--name="+nodeName,
		"--initial-cluster="+initialCluster,
		"--initial-cluster-token="+token,
		"--initial-advertise-peer-urls="+advertise,
	))
}

// run is a convenience wrapper that calls cmd.Run() and returns the error.
func run(cmd exec.Cmd) error {
	return cmd.Run()
}
