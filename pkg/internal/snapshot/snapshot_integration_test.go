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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/internal/integration"
)

// clusterName returns a stable cluster name for this integration test.
// Using t.Name() would create names with slashes; we sanitise to alphanumeric+hyphens.
func integTestClusterName(t *testing.T) string {
	t.Helper()
	// Sanitise test name: replace non-alphanumeric chars with '-', lowercase, limit length.
	raw := strings.ToLower(t.Name())
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	name := b.String()
	if len(name) > 40 {
		name = name[:40]
	}
	// Remove trailing hyphens.
	name = strings.TrimRight(name, "-")
	return "kit-" + name
}

// runKinder runs the kinder binary (built from source via `go run`) with the
// given arguments and returns stdout, stderr, and the exit code.
// Any non-zero exit code is NOT treated as a t.Fatal — callers decide.
func runKinder(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	// Use `go run ./cmd/kind` so we always exercise the current branch source.
	goArgs := append([]string{"run", "./cmd/kind"}, args...)
	cmd := exec.Command("go", goArgs...) //nolint:gosec
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	// Run from repo root — find it relative to the test file.
	cmd.Dir = repoRoot(t)

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			// The process itself couldn't start; treat as exit code 1.
			exitCode = 1
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// runKinderMustSucceed calls runKinder and fatally fails the test if exit code != 0.
func runKinderMustSucceed(t *testing.T, args ...string) string {
	t.Helper()
	stdout, stderr, code := runKinder(t, args...)
	if code != 0 {
		t.Fatalf("kinder %s failed (exit %d)\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), code, stdout, stderr)
	}
	return stdout
}

// runKubectl runs kubectl with the given args, using the kubeconfig from the
// named kinder cluster. Returns stdout; fatally fails on non-zero exit.
func runKubectl(t *testing.T, clusterName string, args ...string) string {
	t.Helper()
	kubecfg := kindKubeconfig(t, clusterName)
	allArgs := append([]string{"--kubeconfig=" + kubecfg}, args...)
	cmd := exec.Command("kubectl", allArgs...) //nolint:gosec
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("kubectl %s failed: %v\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), err, outBuf.String(), errBuf.String())
	}
	return strings.TrimSpace(outBuf.String())
}

// runKubectlWithInput is like runKubectl but pipes manifest text to stdin (-f -).
func runKubectlWithInput(t *testing.T, clusterName string, manifest string, args ...string) {
	t.Helper()
	kubecfg := kindKubeconfig(t, clusterName)
	allArgs := append([]string{"--kubeconfig=" + kubecfg}, args...)
	cmd := exec.Command("kubectl", allArgs...) //nolint:gosec
	cmd.Stdin = strings.NewReader(manifest)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("kubectl %s (with stdin) failed: %v\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), err, outBuf.String(), errBuf.String())
	}
}

// kindKubeconfig returns the path to the kubeconfig for the named kinder cluster.
// kinder stores it at ~/.kube/kind-<cluster>.kubeconfig (or via `kind get kubeconfig`).
// We use `kinder get kubeconfig --name <cluster>` to find it reliably.
func kindKubeconfig(t *testing.T, clusterName string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	// kind/kinder stores kubeconfig at ~/.kube/<cluster>.kubeconfig by convention.
	// Try the standard paths.
	candidates := []string{
		filepath.Join(home, ".kube", "kind-"+clusterName),
		filepath.Join(home, ".kube", clusterName+".kubeconfig"),
		filepath.Join(home, ".kube", "config"), // fallback default
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fallback: ask kinder for it.
	stdout, _, code := runKinder(t, "get", "kubeconfig", "--name", clusterName)
	if code != 0 {
		// Write it to a temp file.
		t.Logf("kinder get kubeconfig exited %d; using default ~/.kube/config", code)
		return filepath.Join(home, ".kube", "config")
	}
	tmpFile := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(tmpFile, []byte(stdout), 0600); err != nil {
		t.Fatalf("write temp kubeconfig: %v", err)
	}
	return tmpFile
}

// repoRoot returns the repository root by walking up from the current file.
func repoRoot(t *testing.T) string {
	t.Helper()
	// The test file lives at pkg/internal/snapshot/ — walk up 3 levels.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Navigate to repo root: pkg/internal/snapshot → pkg/internal → pkg → root.
	root := filepath.Join(dir, "..", "..", "..")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs repoRoot: %v", err)
	}
	return abs
}

// snapshotDir returns ~/.kinder/snapshots/<cluster>.
func snapshotDir(t *testing.T, clusterName string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return filepath.Join(home, ".kinder", "snapshots", clusterName)
}

// waitForPodReady polls until the named pod in the given namespace is Ready,
// up to maxWait duration.
func waitForPodReady(t *testing.T, clusterName, namespace, podName string, maxWait time.Duration) {
	t.Helper()
	kubecfg := kindKubeconfig(t, clusterName)
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		cmd := exec.Command("kubectl", "--kubeconfig="+kubecfg,
			"get", "pod", podName, "-n", namespace,
			"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}") //nolint:gosec
		out, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(out)) == "True" {
			t.Logf("Pod %s/%s is Ready", namespace, podName)
			return
		}
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("pod %s/%s not Ready after %v", namespace, podName, maxWait)
}

// TestIntegrationSnapshotConfigMapRoundTrip is the LIFE-08 happy-path test.
//
// It:
//  1. Creates a real kinder cluster with --local-path.
//  2. Seeds a ConfigMap and a PVC-backed Pod with a sentinel file.
//  3. Takes a snapshot named "golden".
//  4. Asserts LIFE-08 metadata fields are non-empty.
//  5. Mutates the ConfigMap (deletes it) and the PV file (rewrites with different data).
//  6. Restores from "golden".
//  7. Asserts the ConfigMap value and PV file contents match the originals.
func TestIntegrationSnapshotConfigMapRoundTrip(t *testing.T) {
	integration.MaybeSkip(t)

	clusterName := integTestClusterName(t)
	snapName := "golden"
	sentinelMsg := fmt.Sprintf("captured at %s", time.Now().UTC().Format(time.RFC3339))

	// ── Step 1: Create cluster ────────────────────────────────────────────────
	t.Logf("[step 1] Creating cluster %q with --local-path", clusterName)
	runKinderMustSucceed(t, "create", "cluster", "--name", clusterName, "--local-path")

	// Always clean up, even on failure.
	t.Cleanup(func() {
		t.Logf("[cleanup] Deleting cluster %q", clusterName)
		stdout, stderr, code := runKinder(t, "delete", "cluster", "--name", clusterName)
		if code != 0 {
			t.Logf("cleanup: delete cluster exited %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
	})

	// ── Step 2: Seed data ─────────────────────────────────────────────────────
	t.Logf("[step 2] Seeding ConfigMap and PVC-backed Pod")

	cmManifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: kinder-snap-test
  namespace: default
data:
  message: %q
`, sentinelMsg)
	runKubectlWithInput(t, clusterName, cmManifest, "apply", "-f", "-")

	// Create a PVC and Pod that writes a sentinel file.
	pvcManifest := `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: kinder-snap-pvc
  namespace: default
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Mi
  storageClassName: standard
---
apiVersion: v1
kind: Pod
metadata:
  name: kinder-snap-sentinel
  namespace: default
spec:
  restartPolicy: Never
  containers:
  - name: sentinel
    image: busybox:latest
    command: ["/bin/sh", "-c", "echo 'sentinel-original' > /data/sentinel.txt && sleep 3600"]
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: kinder-snap-pvc
`
	runKubectlWithInput(t, clusterName, pvcManifest, "apply", "-f", "-")

	t.Logf("[step 2] Waiting for sentinel pod to be Ready (up to 3 minutes)…")
	waitForPodReady(t, clusterName, "default", "kinder-snap-sentinel", 3*time.Minute)

	// ── Step 3: Take snapshot ─────────────────────────────────────────────────
	t.Logf("[step 3] Creating snapshot %q for cluster %q", snapName, clusterName)
	runKinderMustSucceed(t, "snapshot", "create", clusterName, snapName)

	// Assert archive + sidecar exist.
	archivePath := filepath.Join(snapshotDir(t, clusterName), snapName+".tar.gz")
	sidecarPath := archivePath + ".sha256"
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("[step 3] snapshot archive not found at %s: %v", archivePath, err)
	}
	if _, err := os.Stat(sidecarPath); err != nil {
		t.Fatalf("[step 3] snapshot sidecar not found at %s: %v", sidecarPath, err)
	}
	t.Logf("[step 3] Archive: %s (sidecar present)", archivePath)

	// ── Step 4: LIFE-08 metadata assertions ───────────────────────────────────
	t.Logf("[step 4] Asserting LIFE-08 metadata fields")
	br, err := OpenBundle(archivePath)
	if err != nil {
		t.Fatalf("[step 4] OpenBundle: %v", err)
	}
	defer br.Close()

	meta := br.Metadata()
	if meta == nil {
		t.Fatal("[step 4] metadata is nil inside bundle")
	}
	if meta.K8sVersion == "" {
		t.Error("[step 4] LIFE-08: K8sVersion is empty")
	}
	if meta.NodeImage == "" {
		t.Error("[step 4] LIFE-08: NodeImage is empty")
	}
	if meta.Topology.ControlPlaneCount < 1 {
		t.Errorf("[step 4] LIFE-08: Topology.ControlPlaneCount = %d (want >= 1)", meta.Topology.ControlPlaneCount)
	}
	if len(meta.AddonVersions) < 1 {
		t.Error("[step 4] LIFE-08: AddonVersions is empty (want at least 'localPath')")
	}
	if v, ok := meta.AddonVersions["localPath"]; !ok || v == "" {
		t.Errorf("[step 4] LIFE-08: AddonVersions[\"localPath\"] = %q (want non-empty)", v)
	}
	if len(meta.EtcdDigest) != 64 {
		t.Errorf("[step 4] LIFE-08: EtcdDigest len = %d (want 64 hex chars)", len(meta.EtcdDigest))
	}
	if meta.ImagesDigest == "" {
		t.Error("[step 4] LIFE-08: ImagesDigest is empty")
	}
	if meta.ArchiveDigest == "" {
		// ArchiveDigest in the tarred metadata.json is intentionally left empty (chicken-and-egg);
		// the sidecar file is the source of truth. We verify the sidecar separately.
		t.Logf("[step 4] LIFE-08: ArchiveDigest inside metadata.json is empty (expected — sidecar is the source of truth)")
	}
	t.Logf("[step 4] LIFE-08 OK: k8s=%s nodeImage=%s topo=%+v addons=%v etcdDigest=%s…",
		meta.K8sVersion, meta.NodeImage, meta.Topology, meta.AddonVersions, meta.EtcdDigest[:8])

	// ── Step 5: Mutate ────────────────────────────────────────────────────────
	t.Logf("[step 5] Mutating: deleting ConfigMap and overwriting PV sentinel file")
	runKubectl(t, clusterName, "delete", "configmap", "kinder-snap-test", "-n", "default")

	// Verify ConfigMap is gone.
	kubecfg := kindKubeconfig(t, clusterName)
	checkCmd := exec.Command("kubectl", "--kubeconfig="+kubecfg,
		"get", "configmap", "kinder-snap-test", "-n", "default") //nolint:gosec
	if err := checkCmd.Run(); err == nil {
		t.Fatal("[step 5] ConfigMap still exists after delete")
	}
	t.Logf("[step 5] ConfigMap deleted OK")

	// Overwrite the sentinel file inside the pod.
	runKubectl(t, clusterName, "exec", "-n", "default", "kinder-snap-sentinel",
		"--", "/bin/sh", "-c", "echo 'sentinel-mutated' > /data/sentinel.txt")
	t.Logf("[step 5] PV sentinel overwritten to 'sentinel-mutated'")

	// ── Step 6: Restore ───────────────────────────────────────────────────────
	t.Logf("[step 6] Restoring snapshot %q for cluster %q", snapName, clusterName)
	runKinderMustSucceed(t, "snapshot", "restore", clusterName, snapName)

	// ── Step 7: Assert restoration ────────────────────────────────────────────
	t.Logf("[step 7] Asserting ConfigMap value is restored")
	// Wait up to 60 s for apiserver to be responsive post-restore.
	var cmValue string
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for {
		out, err2 := exec.CommandContext(ctx, "kubectl", //nolint:gosec
			"--kubeconfig="+kubecfg,
			"get", "configmap", "kinder-snap-test", "-n", "default",
			"-o", "jsonpath={.data.message}").Output()
		if err2 == nil {
			cmValue = strings.TrimSpace(string(out))
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("[step 7] ConfigMap not accessible after restore within timeout: %v", ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
	if cmValue != sentinelMsg {
		t.Errorf("[step 7] ConfigMap value = %q; want %q", cmValue, sentinelMsg)
	} else {
		t.Logf("[step 7] ConfigMap value correctly restored: %q", cmValue)
	}

	// Assert PV sentinel file is restored (pod needs to be re-running post-restore).
	t.Logf("[step 7] Asserting PV sentinel file is restored to 'sentinel-original'")
	// Wait for the sentinel pod to be running again post-restore.
	waitForPodReady(t, clusterName, "default", "kinder-snap-sentinel", 2*time.Minute)
	pvContent := runKubectl(t, clusterName, "exec", "-n", "default", "kinder-snap-sentinel",
		"--", "cat", "/data/sentinel.txt")
	if !strings.Contains(pvContent, "sentinel-original") {
		t.Errorf("[step 7] PV sentinel = %q; want to contain 'sentinel-original'", pvContent)
	} else {
		t.Logf("[step 7] PV sentinel correctly restored: %q", pvContent)
	}
}
