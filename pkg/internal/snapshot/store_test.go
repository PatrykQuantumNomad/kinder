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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildFakeBundle writes a minimal valid bundle to the store directory under
// the given name, using synthetic component files. Returns the absolute path
// of the created .tar.gz.
func buildFakeBundle(t *testing.T, store *SnapshotStore, name string, meta *Metadata) string {
	t.Helper()
	compDir := t.TempDir()

	comps := []Component{
		makeComponent(t, compDir, EntryEtcd, []byte("etcd-"+name)),
		makeComponent(t, compDir, EntryImages, []byte("images-"+name)),
		makeComponent(t, compDir, EntryPVs, []byte{}),
		makeComponent(t, compDir, EntryConfig, []byte("cfg: "+name)),
	}

	destPath := store.Path(name)
	if _, err := WriteBundle(context.Background(), destPath, comps, meta); err != nil {
		t.Fatalf("buildFakeBundle(%s): WriteBundle: %v", name, err)
	}
	return destPath
}

// TestStoreList builds three fake bundles, asserts List returns them newest-first,
// and verifies the storage directory has mode 0700.
func TestStoreList(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clusterName := "list-cluster"

	store, err := NewStore(root, clusterName)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Build three bundles with different mtimes. We touch the files after
	// creation to give each a distinct mtime (oldest → newest).
	now := time.Now()
	names := []string{"snap-oldest", "snap-middle", "snap-newest"}
	for i, name := range names {
		meta := &Metadata{
			SchemaVersion: MetadataVersion,
			Name:          name,
			ClusterName:   clusterName,
			CreatedAt:     now.Add(time.Duration(i) * time.Minute),
			K8sVersion:    "v1.31.2",
			NodeImage:     "kindest/node:v1.31.2",
			AddonVersions: map[string]string{},
		}
		archivePath := buildFakeBundle(t, store, name, meta)
		// Set mtime so os.Stat gives us deterministic ordering
		mtime := now.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(archivePath, mtime, mtime); err != nil {
			t.Fatalf("Chtimes %s: %v", name, err)
		}
	}

	infos, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("List: expected 3 snapshots, got %d", len(infos))
	}

	// Newest-first ordering
	wantOrder := []string{"snap-newest", "snap-middle", "snap-oldest"}
	for i, want := range wantOrder {
		if infos[i].Name != want {
			t.Errorf("List[%d]: want %q, got %q", i, want, infos[i].Name)
		}
	}

	// Verify Info fields are populated
	for _, info := range infos {
		if info.Path == "" {
			t.Errorf("info.Path empty for %q", info.Name)
		}
		if info.Size <= 0 {
			t.Errorf("info.Size <= 0 for %q: %d", info.Name, info.Size)
		}
		if info.ClusterName != clusterName {
			t.Errorf("info.ClusterName: want %q, got %q", clusterName, info.ClusterName)
		}
	}

	// Parent storage directory must have mode 0700 (etcd snapshots contain Secrets)
	clusterDir := filepath.Join(root, clusterName)
	fi, err := os.Stat(clusterDir)
	if err != nil {
		t.Fatalf("Stat cluster dir: %v", err)
	}
	if fi.Mode().Perm() != 0700 {
		t.Errorf("cluster dir mode: want 0700, got %04o", fi.Mode().Perm())
	}
}

// TestStoreOpenMissing verifies that opening a non-existent snapshot returns
// ErrSnapshotNotFound.
func TestStoreOpenMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := NewStore(root, "missing-cluster")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, _, err = store.Open(context.Background(), "nope")
	if err == nil {
		t.Fatal("Open: expected ErrSnapshotNotFound, got nil")
	}
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("Open: expected ErrSnapshotNotFound, got %v", err)
	}
}

// TestStoreDelete verifies that Delete removes both .tar.gz and .sha256, and
// that a second Delete returns ErrSnapshotNotFound.
func TestStoreDelete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clusterName := "delete-cluster"
	store, err := NewStore(root, clusterName)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	meta := &Metadata{
		SchemaVersion: MetadataVersion,
		Name:          "snap-to-delete",
		ClusterName:   clusterName,
		K8sVersion:    "v1.31.2",
		NodeImage:     "kindest/node:v1.31.2",
		AddonVersions: map[string]string{},
	}
	buildFakeBundle(t, store, "snap-to-delete", meta)

	// First delete must succeed
	if err := store.Delete(context.Background(), "snap-to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Both files must be gone
	archivePath := store.Path("snap-to-delete")
	sidecarPath := archivePath + ".sha256"
	for _, p := range []string{archivePath, sidecarPath} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected %s to be deleted, got stat err: %v", p, err)
		}
	}

	// Second delete must return ErrSnapshotNotFound
	err = store.Delete(context.Background(), "snap-to-delete")
	if err == nil {
		t.Fatal("second Delete: expected ErrSnapshotNotFound, got nil")
	}
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("second Delete: expected ErrSnapshotNotFound, got %v", err)
	}
}

// TestStoreOpen verifies that Open returns a working BundleReader and Info.
func TestStoreOpen(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clusterName := "open-cluster"
	store, err := NewStore(root, clusterName)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	meta := &Metadata{
		SchemaVersion: MetadataVersion,
		Name:          "snap-open",
		ClusterName:   clusterName,
		K8sVersion:    "v1.31.2",
		NodeImage:     "kindest/node:v1.31.2",
		AddonVersions: map[string]string{"metallb": "v0.13"},
	}
	buildFakeBundle(t, store, "snap-open", meta)

	br, info, err := store.Open(context.Background(), "snap-open")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer br.Close()

	if br.Metadata() == nil {
		t.Error("BundleReader.Metadata() is nil")
	}
	if info.Name != "snap-open" {
		t.Errorf("Info.Name: want %q, got %q", "snap-open", info.Name)
	}
}
