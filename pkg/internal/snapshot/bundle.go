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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// BundleEntries is the canonical ordered list of entry names inside a snapshot
// archive. WriteBundle always writes components in this order, with
// metadata.json appended last.
var BundleEntries = []string{
	EntryEtcd, EntryImages, EntryPVs, EntryConfig, EntryMetadata,
}

// ErrCorruptArchive is returned by VerifyBundle when the archive's sha256 does
// not match the sidecar. Use errors.Is to check.
var ErrCorruptArchive = errors.New("snapshot archive corrupted: sha256 mismatch")

// ErrMissingSidecar is returned by VerifyBundle when the .sha256 sidecar file
// is absent. This is operationally distinct from ErrCorruptArchive: missing
// sidecar may indicate the file was never written (write interrupted) rather
// than a data-integrity failure.
var ErrMissingSidecar = errors.New("snapshot sidecar .sha256 file not found")

// Component represents one host-filesystem file to be packed into the archive
// as a named tar entry. Using a disk path (rather than an io.Reader) lets the
// tar writer know the entry Size before writing, which avoids buffering whole
// components in memory.
type Component struct {
	// Name is one of EntryEtcd | EntryImages | EntryPVs | EntryConfig.
	Name string
	// Path is the absolute host filesystem path containing the raw component bytes.
	Path string
}

// WriteBundle writes destPath (a .tar.gz archive) and destPath+".sha256" (the
// sha256sum-format sidecar file). Components are written in the order supplied;
// metadata.json is appended last. The returned archiveDigest is the hex-encoded
// SHA-256 of the complete gzip stream, computed in a single streaming pass via
// io.MultiWriter so the archive is never read back after writing.
//
// ArchiveDigest in the Metadata embedded inside the tar is intentionally left
// empty — it is a chicken-and-egg problem to include the archive's own digest
// inside it. The returned archiveDigest and the sidecar file are the
// authoritative sources; callers that cache Metadata in memory may populate
// meta.ArchiveDigest with the returned value after WriteBundle returns.
//
// Parent directory creation is the caller's responsibility (SnapshotStore.NewStore
// creates it with mode 0700 before calling WriteBundle).
func WriteBundle(ctx context.Context, destPath string, comps []Component, meta *Metadata) (archiveDigest string, err error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("WriteBundle: context cancelled before start: %w", err)
	}

	f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("WriteBundle: create archive file: %w", err)
	}
	// Close file on any return path; the named return ensures we capture the
	// closer error even on success.
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("WriteBundle: close archive file: %w", cerr)
		}
	}()

	// Single-pass streaming SHA-256: every byte written to gzip also flows
	// through the hasher so we never need to re-read the file.
	h := sha256.New()
	mw := io.MultiWriter(f, h)

	gz, err := gzip.NewWriterLevel(mw, gzip.DefaultCompression)
	if err != nil {
		return "", fmt.Errorf("WriteBundle: create gzip writer: %w", err)
	}
	tw := tar.NewWriter(gz)

	// Write component files
	for _, comp := range comps {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("WriteBundle: context cancelled while writing %s: %w", comp.Name, err)
		}
		if err := writeFileEntry(tw, comp.Name, comp.Path); err != nil {
			return "", fmt.Errorf("WriteBundle: write entry %q: %w", comp.Name, err)
		}
	}

	// Marshal and write metadata.json last. ArchiveDigest inside the tar is ""
	// because we haven't finished hashing yet (recursive problem).
	metaBytes, err := MarshalMetadata(meta)
	if err != nil {
		return "", fmt.Errorf("WriteBundle: marshal metadata: %w", err)
	}
	if err := writeBytesEntry(tw, EntryMetadata, metaBytes); err != nil {
		return "", fmt.Errorf("WriteBundle: write metadata.json entry: %w", err)
	}

	// Flush tar and gzip — this must happen before reading the hash.
	if err := tw.Close(); err != nil {
		return "", fmt.Errorf("WriteBundle: close tar writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("WriteBundle: close gzip writer: %w", err)
	}

	// At this point h contains the sha256 of the complete gzip stream.
	digest := hex.EncodeToString(h.Sum(nil))

	// Write sidecar: sha256sum convention "<hex>  <basename>\n"
	sidecarPath := destPath + ".sha256"
	sidecarContent := fmt.Sprintf("%s  %s\n", digest, filepath.Base(destPath))
	if err := os.WriteFile(sidecarPath, []byte(sidecarContent), 0600); err != nil {
		return "", fmt.Errorf("WriteBundle: write sidecar .sha256: %w", err)
	}

	return digest, nil
}

// writeFileEntry adds a file on disk as a tar entry with the given name.
func writeFileEntry(tw *tar.Writer, entryName, filePath string) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", filePath, err)
	}

	hdr := &tar.Header{
		Name:    entryName,
		Mode:    0600,
		Size:    fi.Size(),
		ModTime: fi.ModTime(),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %s: %w", entryName, err)
	}
	if fi.Size() == 0 {
		return nil // nothing to copy for empty files
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("copy %s into tar: %w", filePath, err)
	}
	return nil
}

// writeBytesEntry adds an in-memory []byte as a tar entry.
func writeBytesEntry(tw *tar.Writer, entryName string, data []byte) error {
	hdr := &tar.Header{
		Name:     entryName,
		Mode:     0600,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %s: %w", entryName, err)
	}
	if _, err := io.Copy(tw, strings.NewReader(string(data))); err != nil {
		return fmt.Errorf("write bytes for %s: %w", entryName, err)
	}
	return nil
}

// VerifyBundle re-hashes archivePath and compares with the sidecar .sha256
// file. Returns:
//   - nil on success
//   - ErrMissingSidecar if the .sha256 file does not exist
//   - ErrCorruptArchive if the sha256 does not match
//   - a wrapped error for I/O failures
func VerifyBundle(archivePath string) error {
	sidecarPath := archivePath + ".sha256"
	sidecarData, err := os.ReadFile(sidecarPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrMissingSidecar
		}
		return fmt.Errorf("VerifyBundle: read sidecar: %w", err)
	}

	// Parse sidecar: "<hex>  <basename>\n"
	line := strings.TrimSpace(string(sidecarData))
	parts := strings.SplitN(line, "  ", 2)
	if len(parts) != 2 {
		return fmt.Errorf("VerifyBundle: malformed sidecar (expected '<hex>  <name>', got %q)", line)
	}
	expectedDigest := strings.TrimSpace(parts[0])

	// Compute actual sha256 of the archive
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("VerifyBundle: open archive: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("VerifyBundle: hash archive: %w", err)
	}
	actualDigest := hex.EncodeToString(h.Sum(nil))

	if actualDigest != expectedDigest {
		return ErrCorruptArchive
	}
	return nil
}

// bundleReader implements BundleReader. It holds the fully-decompressed tar
// index in memory (entry name → []byte) so callers can Open() entries in any
// order. For Plan 03's restore path this is acceptable because the largest
// entries (images.tar, pvs.tar) are extracted to temp files anyway, and the
// in-memory map is populated once on OpenBundle.
type bundleReader struct {
	meta    *Metadata
	entries map[string][]byte
}

// BundleReader streams named entries from an open snapshot archive. Callers
// must call Close when done.
type BundleReader interface {
	// Metadata returns the parsed metadata.json from inside the archive.
	// Returns nil if metadata.json could not be parsed.
	Metadata() *Metadata
	// Open returns a ReadCloser for the named entry (e.g. EntryEtcd).
	// Returns an error if the entry does not exist.
	Open(entryName string) (io.ReadCloser, error)
	// Close releases resources held by this reader.
	Close() error
}

// OpenBundle reads archivePath into a BundleReader. The entire archive is
// decompressed and indexed in memory so callers can Open() entries in any
// order without seeking.
func OpenBundle(archivePath string) (BundleReader, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("OpenBundle: open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("OpenBundle: create gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	entries := make(map[string][]byte)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("OpenBundle: read tar header: %w", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("OpenBundle: read entry %q: %w", hdr.Name, err)
		}
		entries[hdr.Name] = data
	}

	// Parse metadata if present
	var meta *Metadata
	if mdata, ok := entries[EntryMetadata]; ok {
		meta, _ = UnmarshalMetadata(mdata) // best-effort
	}

	return &bundleReader{meta: meta, entries: entries}, nil
}

func (br *bundleReader) Metadata() *Metadata { return br.meta }

func (br *bundleReader) Open(entryName string) (io.ReadCloser, error) {
	data, ok := br.entries[entryName]
	if !ok {
		return nil, fmt.Errorf("bundle entry %q not found", entryName)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (br *bundleReader) Close() error { return nil }
