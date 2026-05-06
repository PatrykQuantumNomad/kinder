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
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeComponent writes data to a temp file and returns a Component for it.
func makeComponent(t *testing.T, dir, name string, data []byte) Component {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("makeComponent: write %s: %v", path, err)
	}
	return Component{Name: name, Path: path}
}

// testMetadata returns a Metadata suitable for testing.
func testMetadata(clusterName string) *Metadata {
	return &Metadata{
		SchemaVersion: MetadataVersion,
		Name:          "snap-test",
		ClusterName:   clusterName,
		CreatedAt:     time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		K8sVersion:    "v1.31.2",
		NodeImage:     "kindest/node:v1.31.2",
		Topology: TopologyInfo{
			ControlPlaneCount: 1,
			WorkerCount:       0,
			HasLoadBalancer:   false,
		},
		AddonVersions: map[string]string{},
	}
}

// TestBundleRoundTrip writes a bundle with 4 small components (one of which is
// an empty pvs.tar), verifies its sidecar integrity, then opens the archive and
// checks that each entry's bytes round-trip exactly.
func TestBundleRoundTrip(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	compDir := t.TempDir()

	// Build deterministic random bytes for etcd.snap
	etcdData := make([]byte, 10)
	rand.New(rand.NewSource(42)).Read(etcdData)

	// 64 KB pseudo-random image blob
	imageData := make([]byte, 64*1024)
	rand.New(rand.NewSource(99)).Read(imageData)

	// pvs.tar is intentionally empty (no PVs)
	pvsData := []byte{}

	// kind-config.yaml is a tiny YAML string
	cfgData := []byte("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\n")

	comps := []Component{
		makeComponent(t, compDir, EntryEtcd, etcdData),
		makeComponent(t, compDir, EntryImages, imageData),
		makeComponent(t, compDir, EntryPVs, pvsData),
		makeComponent(t, compDir, EntryConfig, cfgData),
	}

	meta := testMetadata("roundtrip-cluster")
	destPath := filepath.Join(tmp, "snap-test.tar.gz")

	archiveDigest, err := WriteBundle(context.Background(), destPath, comps, meta)
	if err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}
	if archiveDigest == "" {
		t.Fatal("WriteBundle: returned empty archiveDigest")
	}

	// Sidecar must exist
	sidecarPath := destPath + ".sha256"
	if _, err := os.Stat(sidecarPath); err != nil {
		t.Fatalf("sidecar .sha256 missing: %v", err)
	}

	// VerifyBundle must succeed on the untouched archive
	if err := VerifyBundle(destPath); err != nil {
		t.Fatalf("VerifyBundle on clean archive: %v", err)
	}

	// OpenBundle and check each entry
	br, err := OpenBundle(destPath)
	if err != nil {
		t.Fatalf("OpenBundle: %v", err)
	}
	defer br.Close()

	check := func(entryName string, want []byte) {
		t.Helper()
		rc, err := br.Open(entryName)
		if err != nil {
			t.Fatalf("Open(%q): %v", entryName, err)
		}
		defer rc.Close()
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll(%q): %v", entryName, err)
		}
		if !bytes.Equal(want, got) {
			t.Errorf("entry %q: content mismatch (want %d bytes, got %d bytes)", entryName, len(want), len(got))
		}
	}

	check(EntryEtcd, etcdData)
	check(EntryImages, imageData)
	check(EntryPVs, pvsData)
	check(EntryConfig, cfgData)

	// metadata.json must be readable and parseable
	mrc, err := br.Open(EntryMetadata)
	if err != nil {
		t.Fatalf("Open(metadata.json): %v", err)
	}
	defer mrc.Close()
	mdata, err := io.ReadAll(mrc)
	if err != nil {
		t.Fatalf("ReadAll(metadata.json): %v", err)
	}
	parsedMeta, err := UnmarshalMetadata(mdata)
	if err != nil {
		t.Fatalf("UnmarshalMetadata from bundle: %v", err)
	}
	if parsedMeta.ClusterName != meta.ClusterName {
		t.Errorf("metadata ClusterName: want %q, got %q", meta.ClusterName, parsedMeta.ClusterName)
	}

	// BundleReader.Metadata() must return non-nil
	if br.Metadata() == nil {
		t.Error("BundleReader.Metadata() returned nil")
	}
}

// TestBundleCorruptionDetected verifies that flipping a single byte in the
// middle of the .tar.gz causes VerifyBundle to return ErrCorruptArchive.
func TestBundleCorruptionDetected(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	compDir := t.TempDir()

	comps := []Component{
		makeComponent(t, compDir, EntryEtcd, []byte("etcd-data")),
		makeComponent(t, compDir, EntryImages, []byte("image-data")),
		makeComponent(t, compDir, EntryPVs, []byte{}),
		makeComponent(t, compDir, EntryConfig, []byte("cfg: v1")),
	}

	destPath := filepath.Join(tmp, "snap-corrupt.tar.gz")
	if _, err := WriteBundle(context.Background(), destPath, comps, testMetadata("corrupt-cluster")); err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}

	// Flip one byte in the middle of the archive
	raw, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(raw) < 10 {
		t.Fatalf("archive too small to corrupt: %d bytes", len(raw))
	}
	mid := len(raw) / 2
	raw[mid] ^= 0xFF
	if err := os.WriteFile(destPath, raw, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err = VerifyBundle(destPath)
	if err == nil {
		t.Fatal("VerifyBundle: expected ErrCorruptArchive, got nil")
	}
	if !errors.Is(err, ErrCorruptArchive) {
		t.Fatalf("VerifyBundle: expected ErrCorruptArchive, got %v", err)
	}
}

// TestBundleMissingSidecar verifies that deleting the .sha256 sidecar causes
// VerifyBundle to return ErrMissingSidecar (not ErrCorruptArchive — these are
// semantically distinct: missing sidecar is an operational error, not a
// data-integrity error).
func TestBundleMissingSidecar(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	compDir := t.TempDir()

	comps := []Component{
		makeComponent(t, compDir, EntryEtcd, []byte("etcd")),
		makeComponent(t, compDir, EntryImages, []byte("images")),
		makeComponent(t, compDir, EntryPVs, []byte{}),
		makeComponent(t, compDir, EntryConfig, []byte("cfg")),
	}

	destPath := filepath.Join(tmp, "snap-nosidecar.tar.gz")
	if _, err := WriteBundle(context.Background(), destPath, comps, testMetadata("nosidecar-cluster")); err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}

	// Delete the sidecar
	sidecarPath := destPath + ".sha256"
	if err := os.Remove(sidecarPath); err != nil {
		t.Fatalf("Remove sidecar: %v", err)
	}

	err := VerifyBundle(destPath)
	if err == nil {
		t.Fatal("VerifyBundle: expected error with missing sidecar, got nil")
	}
	if !errors.Is(err, ErrMissingSidecar) {
		t.Fatalf("VerifyBundle: expected ErrMissingSidecar, got %v (type %T)", err, err)
	}
	// Must NOT be ErrCorruptArchive — different sentinel
	if errors.Is(err, ErrCorruptArchive) {
		t.Fatal("VerifyBundle: ErrMissingSidecar should not wrap ErrCorruptArchive")
	}
}

// TestBundleEmptyPVs verifies that a pvs.tar component with size=0 is written
// and read back cleanly: Open("pvs.tar") returns an io.ReadCloser that yields
// EOF immediately with no error.
func TestBundleEmptyPVs(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	compDir := t.TempDir()

	comps := []Component{
		makeComponent(t, compDir, EntryEtcd, []byte("etcd")),
		makeComponent(t, compDir, EntryImages, []byte("img")),
		makeComponent(t, compDir, EntryPVs, []byte{}), // empty
		makeComponent(t, compDir, EntryConfig, []byte("cfg")),
	}

	destPath := filepath.Join(tmp, "snap-emptypvs.tar.gz")
	if _, err := WriteBundle(context.Background(), destPath, comps, testMetadata("emptypvs-cluster")); err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}

	br, err := OpenBundle(destPath)
	if err != nil {
		t.Fatalf("OpenBundle: %v", err)
	}
	defer br.Close()

	rc, err := br.Open(EntryPVs)
	if err != nil {
		t.Fatalf("Open(pvs.tar): %v", err)
	}
	defer rc.Close()

	buf := make([]byte, 1)
	n, err := rc.Read(buf)
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Errorf("expected (0, io.EOF) for empty pvs.tar, got (%d, %v)", n, err)
	}
}

// TestBundleContextCancellation verifies that WriteBundle respects context
// cancellation before it starts writing.
func TestBundleContextCancellation(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	compDir := t.TempDir()

	comps := []Component{
		makeComponent(t, compDir, EntryEtcd, []byte("etcd")),
		makeComponent(t, compDir, EntryImages, []byte("img")),
		makeComponent(t, compDir, EntryPVs, []byte{}),
		makeComponent(t, compDir, EntryConfig, []byte("cfg")),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	destPath := filepath.Join(tmp, "snap-cancelled.tar.gz")
	_, err := WriteBundle(ctx, destPath, comps, testMetadata("cancelled-cluster"))
	if err == nil {
		t.Fatal("WriteBundle: expected error on cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context") && !errors.Is(err, context.Canceled) {
		t.Logf("WriteBundle with cancelled ctx returned: %v (acceptable non-context error)", err)
	}
}
