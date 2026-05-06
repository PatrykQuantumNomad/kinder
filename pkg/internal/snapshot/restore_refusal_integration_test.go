// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/internal/integration"
)

// cpContainerName returns the Docker container name for the control-plane node
// of the named kinder cluster. kinder uses the convention "<cluster>-control-plane".
func cpContainerName(clusterName string) string {
	return clusterName + "-control-plane"
}

// workerContainerName returns the Docker container name for worker N (1-indexed)
// of the named kinder cluster. kinder uses the convention "<cluster>-worker<N>".
func workerContainerName(clusterName string, n int) string {
	if n == 1 {
		return clusterName + "-worker"
	}
	return fmt.Sprintf("%s-worker%d", clusterName, n)
}

// runDockerMustSucceed runs a docker command and fatally fails on non-zero exit.
func runDockerMustSucceed(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s failed: %v\noutput: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// assertKinderRestoreFails runs `kinder snapshot restore <cluster> <snap>` and
// asserts it exits non-zero and that stderr (lowercased) contains all of wantInStderr.
// It also asserts that none of noneOfInStderr appear in the lowercase stderr.
func assertKinderRestoreFails(t *testing.T, clusterName, snapName string,
	wantInStderr []string, noneOfInStderr []string) {
	t.Helper()
	stdout, stderr, code := runKinder(t, "snapshot", "restore", clusterName, snapName)
	if code == 0 {
		t.Fatalf("kinder snapshot restore exited 0 (want non-zero)\nstdout: %s\nstderr: %s",
			stdout, stderr)
	}
	lowerStderr := strings.ToLower(stderr)
	t.Logf("kinder snapshot restore exited %d (expected). stderr: %s", code, stderr)
	for _, want := range wantInStderr {
		if !strings.Contains(lowerStderr, strings.ToLower(want)) {
			t.Errorf("stderr missing %q\nfull stderr: %s", want, stderr)
		}
	}
	for _, none := range noneOfInStderr {
		if strings.Contains(lowerStderr, strings.ToLower(none)) {
			t.Errorf("stderr unexpectedly contains %q (only the intended dimension should fail)\nfull stderr: %s",
				none, stderr)
		}
	}
}

// TestIntegrationRestoreRefusesOnK8sMismatch verifies SC2 hard-fail:
// a snapshot taken while the cluster reports K8s version V cannot be restored to
// a cluster that now reports a different version. We simulate the mismatch by
// editing /kind/version inside the CP container after taking the snapshot.
func TestIntegrationRestoreRefusesOnK8sMismatch(t *testing.T) {
	integration.MaybeSkip(t)

	clusterName := integTestClusterName(t)
	snapName := "v1"

	// Create cluster.
	t.Logf("[k8s-mismatch] Creating cluster %q", clusterName)
	runKinderMustSucceed(t, "create", "cluster", "--name", clusterName)
	t.Cleanup(func() {
		t.Logf("[k8s-mismatch] Deleting cluster %q", clusterName)
		runKinder(t, "delete", "cluster", "--name", clusterName) //nolint:errcheck
	})

	// Take snapshot.
	t.Logf("[k8s-mismatch] Taking snapshot %q", snapName)
	runKinderMustSucceed(t, "snapshot", "create", clusterName, snapName)

	// Tamper: overwrite /kind/version on the running CP container.
	cpContainer := cpContainerName(clusterName)
	t.Logf("[k8s-mismatch] Tampering: echo v9.99.99 > /kind/version on %q", cpContainer)
	runDockerMustSucceed(t, "exec", cpContainer, "sh", "-c", "echo v9.99.99 > /kind/version")

	// Verify the tamper took effect.
	versionOut := runDockerMustSucceed(t, "exec", cpContainer, "cat", "/kind/version")
	if !strings.Contains(versionOut, "v9.99.99") {
		t.Fatalf("[k8s-mismatch] tamper did not take effect; /kind/version = %q", versionOut)
	}
	t.Logf("[k8s-mismatch] /kind/version is now %q", versionOut)

	// Restore must refuse.
	t.Logf("[k8s-mismatch] Asserting restore refuses with k8s version mismatch")
	assertKinderRestoreFails(t, clusterName, snapName,
		[]string{"k8s version mismatch"},
		nil, // no exclusions
	)
}

// TestIntegrationRestoreRefusesOnTopologyMismatch verifies the CONTEXT.md
// topology hard-fail: a snapshot of a 1-CP+2-worker cluster cannot be restored
// to a cluster that now has only 1-CP+1-worker (because we removed a worker).
//
// We create a 2-node cluster (CP + worker) and remove the worker container via
// `docker rm -f` so that provider.ListNodes returns only the CP, creating a
// node-count mismatch.
func TestIntegrationRestoreRefusesOnTopologyMismatch(t *testing.T) {
	integration.MaybeSkip(t)

	clusterName := integTestClusterName(t)
	snapName := "topo"

	// Write a multi-node kind config under TempDir.
	kindCfg := filepath.Join(t.TempDir(), "kind-topo.yaml")
	kindCfgContent := `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
`
	if err := os.WriteFile(kindCfg, []byte(kindCfgContent), 0600); err != nil {
		t.Fatalf("write kind config: %v", err)
	}

	// Create 2-node cluster.
	t.Logf("[topo-mismatch] Creating 2-node cluster %q (1 CP + 1 worker)", clusterName)
	runKinderMustSucceed(t, "create", "cluster", "--name", clusterName, "--config", kindCfg)
	t.Cleanup(func() {
		t.Logf("[topo-mismatch] Deleting cluster %q", clusterName)
		runKinder(t, "delete", "cluster", "--name", clusterName) //nolint:errcheck
	})

	// Take snapshot of 2-node topology.
	t.Logf("[topo-mismatch] Taking snapshot %q of 2-node cluster", snapName)
	runKinderMustSucceed(t, "snapshot", "create", clusterName, snapName)

	// Tamper: remove the worker container so ListNodes returns only the CP.
	workerContainer := workerContainerName(clusterName, 1)
	t.Logf("[topo-mismatch] Removing worker container %q to create topology mismatch", workerContainer)
	runDockerMustSucceed(t, "rm", "-f", workerContainer)

	// Brief pause to ensure docker state is consistent.
	time.Sleep(2 * time.Second)

	// Restore must refuse because snapshot has workerCount=1 but live has workerCount=0.
	t.Logf("[topo-mismatch] Asserting restore refuses with topology mismatch")
	assertKinderRestoreFails(t, clusterName, snapName,
		[]string{"topology mismatch"},
		nil,
	)
}

// TestIntegrationRestoreRefusesOnAddonMismatch verifies the CONTEXT.md
// addon-version hard-fail: the snapshot's addonVersions["localPath"] value
// differs from the live cluster's local-path-provisioner image after we bump
// the deployment image to a fake tag.
func TestIntegrationRestoreRefusesOnAddonMismatch(t *testing.T) {
	integration.MaybeSkip(t)

	clusterName := integTestClusterName(t)
	snapName := "addon-base"

	// Create cluster with --local-path so the addon is installed.
	t.Logf("[addon-mismatch] Creating cluster %q with --local-path", clusterName)
	runKinderMustSucceed(t, "create", "cluster", "--name", clusterName, "--local-path")
	t.Cleanup(func() {
		t.Logf("[addon-mismatch] Deleting cluster %q", clusterName)
		runKinder(t, "delete", "cluster", "--name", clusterName) //nolint:errcheck
	})

	// Take snapshot — capture the current localPath addon version.
	t.Logf("[addon-mismatch] Taking snapshot %q", snapName)
	runKinderMustSucceed(t, "snapshot", "create", clusterName, snapName)

	// Confirm metadata.json captured addonVersions["localPath"].
	archivePath := filepath.Join(snapshotDir(t, clusterName), snapName+".tar.gz")
	br, err := OpenBundle(archivePath)
	if err != nil {
		t.Fatalf("[addon-mismatch] OpenBundle: %v", err)
	}
	meta := br.Metadata()
	br.Close()
	if meta == nil {
		t.Fatal("[addon-mismatch] metadata is nil")
	}
	if v, ok := meta.AddonVersions["localPath"]; !ok || v == "" {
		t.Fatalf("[addon-mismatch] addonVersions[\"localPath\"] = %q (want non-empty after snapshot)", v)
	}
	t.Logf("[addon-mismatch] snapshot addonVersions[\"localPath\"] = %q", meta.AddonVersions["localPath"])

	// Tamper: bump the local-path-provisioner deployment image to a fake tag.
	t.Logf("[addon-mismatch] Bumping local-path-provisioner to fake tag v0.0.99")
	runKubectl(t, clusterName,
		"-n", "local-path-storage",
		"set", "image",
		"deployment/local-path-provisioner",
		"local-path-provisioner=rancher/local-path-provisioner:v0.0.99",
	)
	// Wait for rollout (may not complete since v0.0.99 doesn't exist, but the
	// deployment image field is updated immediately — CaptureAddonVersions reads
	// from the spec, not the running pod image).
	t.Logf("[addon-mismatch] Waiting briefly for deployment spec to reflect new image…")
	time.Sleep(5 * time.Second)

	// Verify the deployment now shows v0.0.99.
	currentImage := runKubectl(t, clusterName,
		"-n", "local-path-storage",
		"get", "deployment", "local-path-provisioner",
		"-o", `jsonpath={.spec.template.spec.containers[0].image}`,
	)
	t.Logf("[addon-mismatch] live local-path-provisioner image = %q", currentImage)
	if !strings.Contains(currentImage, "v0.0.99") {
		t.Fatalf("[addon-mismatch] deployment image not updated; got %q", currentImage)
	}

	// Restore must refuse because addon versions differ.
	t.Logf("[addon-mismatch] Asserting restore refuses with addon version mismatch")
	assertKinderRestoreFails(t, clusterName, snapName,
		[]string{"addon"},
		[]string{"k8s version mismatch", "topology mismatch"},
	)
}

// TestIntegrationListShowsCorrupt verifies that `kinder snapshot list` shows
// STATUS="corrupt" for a snapshot whose .tar.gz has been tampered (single byte
// flip at offset 512) while the .sha256 sidecar remains with the original digest.
func TestIntegrationListShowsCorrupt(t *testing.T) {
	integration.MaybeSkip(t)

	clusterName := integTestClusterName(t)
	snapName := "victim"

	// Create cluster.
	t.Logf("[list-corrupt] Creating cluster %q", clusterName)
	runKinderMustSucceed(t, "create", "cluster", "--name", clusterName)
	t.Cleanup(func() {
		t.Logf("[list-corrupt] Deleting cluster %q", clusterName)
		runKinder(t, "delete", "cluster", "--name", clusterName) //nolint:errcheck
	})

	// Take snapshot.
	t.Logf("[list-corrupt] Taking snapshot %q", snapName)
	runKinderMustSucceed(t, "snapshot", "create", clusterName, snapName)

	// Locate archive.
	archivePath := filepath.Join(snapshotDir(t, clusterName), snapName+".tar.gz")
	sidecarPath := archivePath + ".sha256"

	// Sanity: sidecar exists.
	if _, err := os.Stat(sidecarPath); err != nil {
		t.Fatalf("[list-corrupt] sidecar not found: %v", err)
	}

	// Flip byte 512 in the archive. Use a 1 KiB read-modify-write.
	t.Logf("[list-corrupt] Flipping byte at offset 512 in %s", archivePath)
	if err := flipByteAtOffset(archivePath, 512); err != nil {
		t.Fatalf("[list-corrupt] flipByteAtOffset: %v", err)
	}

	// `kinder snapshot list` should now show STATUS="corrupt".
	t.Logf("[list-corrupt] Running `kinder snapshot list %s`", clusterName)
	stdout, stderr, code := runKinder(t, "snapshot", "list", clusterName)
	if code != 0 {
		t.Fatalf("[list-corrupt] kinder snapshot list exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	t.Logf("[list-corrupt] list output:\n%s", stdout)

	if !strings.Contains(stdout, snapName) {
		t.Errorf("[list-corrupt] snapshot %q not found in list output:\n%s", snapName, stdout)
	}
	if !strings.Contains(strings.ToLower(stdout), "corrupt") {
		t.Errorf("[list-corrupt] STATUS 'corrupt' not found in list output:\n%s", stdout)
	}

	// Also assert JSON output carries Status="corrupt".
	t.Logf("[list-corrupt] Running `kinder snapshot list --output json %s`", clusterName)
	jsonOut, jsonErr, jsonCode := runKinder(t, "snapshot", "list", "--output", "json", clusterName)
	if jsonCode != 0 {
		t.Fatalf("[list-corrupt] kinder snapshot list --output json exited %d\nstdout: %s\nstderr: %s",
			jsonCode, jsonOut, jsonErr)
	}
	if !strings.Contains(strings.ToLower(jsonOut), `"corrupt"`) &&
		!strings.Contains(strings.ToLower(jsonOut), `"status":"corrupt"`) &&
		!strings.Contains(strings.ToLower(jsonOut), `corrupt`) {
		t.Errorf("[list-corrupt] JSON output does not contain corrupt status:\n%s", jsonOut)
	}
	t.Logf("[list-corrupt] JSON output:\n%s", jsonOut)
}

// flipByteAtOffset flips one byte (XOR 0xFF) at the given offset in a file.
// If the file is shorter than offset+1, the function returns an error.
func flipByteAtOffset(path string, offset int64) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 1)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return fmt.Errorf("read at offset %d: %w", offset, err)
	}
	buf[0] ^= 0xFF
	if _, err := f.WriteAt(buf, offset); err != nil {
		return fmt.Errorf("write at offset %d: %w", offset, err)
	}
	return nil
}
