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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrSnapshotNotFound is returned by Open and Delete when the named snapshot
// does not exist in the store.
var ErrSnapshotNotFound = errors.New("snapshot not found")

// Info describes a snapshot on disk. It is the element type of the slice
// returned by SnapshotStore.List and the input type consumed by prune policy
// functions.
type Info struct {
	// Name is the snapshot's short name (without .tar.gz suffix).
	Name string
	// ClusterName is the owning cluster, taken from the store's cluster directory.
	ClusterName string
	// Path is the absolute path to the .tar.gz file.
	Path string
	// Size is the combined size of the .tar.gz and .sha256 sidecar files in bytes.
	Size int64
	// CreatedAt is the mtime of the .tar.gz file. For snapshots written by
	// WriteBundle the mtime equals the snapshot creation time.
	CreatedAt time.Time
	// Metadata holds the parsed metadata.json from inside the archive. May be
	// nil if the archive could not be opened (e.g., truncated write).
	Metadata *Metadata
	// Status is "ok" | "corrupt" | "unknown".
	//   "ok":      sidecar .sha256 exists and full re-hash matches.
	//   "corrupt": sidecar exists but re-hash mismatches.
	//   "unknown": sidecar missing or archive could not be read.
	// List uses full re-hash (VerifyBundle). For the CLI list command, callers
	// that want a fast path can call ListVerify with StatusFast mode
	// (see Plan 05). The default here is accurate.
	Status string
}

// SnapshotStore manages snapshots for a single cluster under a root directory.
// The on-disk layout is: <root>/<clusterName>/<name>.tar.gz (+ .sha256 sidecar).
type SnapshotStore struct {
	root        string // <root>/<clusterName>/
	clusterName string
}

// DefaultRoot returns ~/.kinder/snapshots — the default parent directory for
// all snapshot stores. Callers append the cluster name to obtain the full path.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("snapshot: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".kinder", "snapshots"), nil
}

// NewStore creates (if needed) the per-cluster directory under root and returns
// a *SnapshotStore. If root is empty, DefaultRoot is used.
//
// The parent root directory and the per-cluster subdirectory are created with
// mode 0700 — etcd snapshots contain Secrets and must not be world-readable.
func NewStore(root, clusterName string) (*SnapshotStore, error) {
	if root == "" {
		var err error
		root, err = DefaultRoot()
		if err != nil {
			return nil, err
		}
	}
	if clusterName == "" {
		return nil, fmt.Errorf("snapshot: NewStore: clusterName is empty")
	}

	clusterDir := filepath.Join(root, clusterName)
	if err := os.MkdirAll(clusterDir, 0700); err != nil {
		return nil, fmt.Errorf("snapshot: NewStore: create store dir %q: %w", clusterDir, err)
	}

	return &SnapshotStore{root: clusterDir, clusterName: clusterName}, nil
}

// Path returns the absolute path of the .tar.gz file for the given snapshot name.
func (s *SnapshotStore) Path(name string) string {
	return filepath.Join(s.root, name+".tar.gz")
}

// SnapshotRoot returns the per-cluster directory managed by this store.
func (s *SnapshotStore) SnapshotRoot() string { return s.root }

// List returns all snapshots in the store, sorted newest-first by mtime.
// For each snapshot, List:
//   1. Stats the .tar.gz file for size + mtime.
//   2. Stats the .sha256 sidecar to include its size in Info.Size.
//   3. Opens the archive via OpenBundle to read metadata.json.
//   4. Calls VerifyBundle for an accurate Status ("ok" | "corrupt" | "unknown").
//
// List is context-aware: it checks ctx.Err() between snapshots and returns
// early if the context is cancelled.
func (s *SnapshotStore) List(ctx context.Context) ([]Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("SnapshotStore.List: context cancelled: %w", err)
	}

	pattern := filepath.Join(s.root, "*.tar.gz")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("SnapshotStore.List: glob %q: %w", pattern, err)
	}

	infos := make([]Info, 0, len(matches))
	for _, archivePath := range matches {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("SnapshotStore.List: context cancelled: %w", err)
		}

		info, err := s.infoFor(archivePath)
		if err != nil {
			// Best-effort: skip unreadable archives rather than aborting the whole list.
			continue
		}
		infos = append(infos, info)
	}

	// Sort newest-first by CreatedAt (mtime).
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].CreatedAt.After(infos[j].CreatedAt)
	})

	return infos, nil
}

// infoFor builds an Info for the given .tar.gz file.
func (s *SnapshotStore) infoFor(archivePath string) (Info, error) {
	fi, err := os.Stat(archivePath)
	if err != nil {
		return Info{}, fmt.Errorf("stat archive: %w", err)
	}

	// Strip .tar.gz suffix to get the snapshot name.
	base := filepath.Base(archivePath)
	name := strings.TrimSuffix(base, ".tar.gz")

	size := fi.Size()
	// Add sidecar size if it exists.
	if sf, err := os.Stat(archivePath + ".sha256"); err == nil {
		size += sf.Size()
	}

	info := Info{
		Name:        name,
		ClusterName: s.clusterName,
		Path:        archivePath,
		Size:        size,
		CreatedAt:   fi.ModTime(),
	}

	// Best-effort: read metadata from the archive.
	if br, err := OpenBundle(archivePath); err == nil {
		info.Metadata = br.Metadata()
		br.Close()
	}

	// Status via VerifyBundle (full re-hash).
	switch err := VerifyBundle(archivePath); {
	case err == nil:
		info.Status = "ok"
	case errors.Is(err, ErrCorruptArchive):
		info.Status = "corrupt"
	default:
		info.Status = "unknown"
	}

	return info, nil
}

// Open returns a BundleReader and Info for the named snapshot.
// Returns ErrSnapshotNotFound if the snapshot does not exist.
func (s *SnapshotStore) Open(ctx context.Context, name string) (BundleReader, *Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("SnapshotStore.Open: context cancelled: %w", err)
	}

	archivePath := s.Path(name)
	if _, err := os.Stat(archivePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrSnapshotNotFound
		}
		return nil, nil, fmt.Errorf("SnapshotStore.Open: stat %q: %w", archivePath, err)
	}

	br, err := OpenBundle(archivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("SnapshotStore.Open: open bundle %q: %w", name, err)
	}

	info, err := s.infoFor(archivePath)
	if err != nil {
		br.Close()
		return nil, nil, fmt.Errorf("SnapshotStore.Open: info for %q: %w", name, err)
	}

	return br, &info, nil
}

// Delete removes the .tar.gz and .sha256 sidecar for the named snapshot.
// Returns ErrSnapshotNotFound if neither file exists.
func (s *SnapshotStore) Delete(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("SnapshotStore.Delete: context cancelled: %w", err)
	}

	archivePath := s.Path(name)
	sidecarPath := archivePath + ".sha256"

	// Check existence first for the ErrSnapshotNotFound sentinel.
	if _, err := os.Stat(archivePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrSnapshotNotFound
		}
		return fmt.Errorf("SnapshotStore.Delete: stat %q: %w", archivePath, err)
	}

	if err := os.Remove(archivePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SnapshotStore.Delete: remove archive %q: %w", archivePath, err)
	}
	if err := os.Remove(sidecarPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SnapshotStore.Delete: remove sidecar %q: %w", sidecarPath, err)
	}
	return nil
}
