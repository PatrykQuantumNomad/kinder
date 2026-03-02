package kube

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/kind/pkg/log"
)

// writeTarGz creates an in-memory gzipped tarball from the given entries and
// writes it to a temporary file, returning the file path.
func writeTarGz(t *testing.T, entries []tarEntry) string {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		hdr := &tar.Header{
			Name: e.name,
			Mode: 0o644,
			Size: int64(len(e.body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("writing tar header for %q: %v", e.name, err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatalf("writing tar body for %q: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := os.WriteFile(tmpFile, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("writing temp tar.gz file: %v", err)
	}
	return tmpFile
}

type tarEntry struct {
	name string
	body string
}

func TestExtractTarball_Normal(t *testing.T) {
	entries := []tarEntry{
		{name: "dir/file1.txt", body: "hello"},
		{name: "dir/sub/file2.txt", body: "world"},
		{name: "root.txt", body: "root level"},
	}
	tarPath := writeTarGz(t, entries)
	destDir := t.TempDir()

	logger := log.NoopLogger{}
	if err := extractTarball(tarPath, destDir, logger); err != nil {
		t.Fatalf("extractTarball returned unexpected error: %v", err)
	}

	// Verify all files were extracted with correct content.
	for _, e := range entries {
		path := filepath.Join(destDir, e.name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected file %q to exist: %v", path, err)
			continue
		}
		if string(data) != e.body {
			t.Errorf("file %q: got content %q, want %q", path, string(data), e.body)
		}
	}
}

func TestExtractTarball_PathTraversal(t *testing.T) {
	entries := []tarEntry{
		{name: "../../etc/evil", body: "malicious"},
	}
	tarPath := writeTarGz(t, entries)
	destDir := t.TempDir()

	logger := log.NoopLogger{}
	err := extractTarball(tarPath, destDir, logger)
	if err == nil {
		t.Fatal("extractTarball should have returned an error for path traversal entry")
	}

	// Verify the evil file was NOT created outside destDir.
	evilPath := filepath.Join(destDir, "..", "..", "etc", "evil")
	if _, statErr := os.Stat(evilPath); statErr == nil {
		t.Errorf("path traversal file was created at %q", evilPath)
	}
}

func TestExtractTarball_AbsolutePath(t *testing.T) {
	entries := []tarEntry{
		{name: "/etc/evil", body: "malicious"},
	}
	tarPath := writeTarGz(t, entries)
	destDir := t.TempDir()

	logger := log.NoopLogger{}
	err := extractTarball(tarPath, destDir, logger)
	if err == nil {
		t.Fatal("extractTarball should have returned an error for absolute path entry")
	}
}

func TestSanitizeTarPath(t *testing.T) {
	destDir := "/tmp/extract"

	tests := []struct {
		name     string
		entry    string
		wantErr  bool
		wantPath string
	}{
		{
			name:     "normal relative path",
			entry:    "dir/file.txt",
			wantErr:  false,
			wantPath: filepath.Join(destDir, "dir/file.txt"),
		},
		{
			name:     "simple filename",
			entry:    "file.txt",
			wantErr:  false,
			wantPath: filepath.Join(destDir, "file.txt"),
		},
		{
			name:    "path traversal with dot-dot",
			entry:   "../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "absolute path",
			entry:   "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "hidden traversal via dot-dot in middle",
			entry:   "foo/../../etc/passwd",
			wantErr: true,
		},
		{
			name:     "dot-dot that stays within dest",
			entry:    "foo/../bar/file.txt",
			wantErr:  false,
			wantPath: filepath.Join(destDir, "bar/file.txt"),
		},
		{
			name:     "current directory reference",
			entry:    "./file.txt",
			wantErr:  false,
			wantPath: filepath.Join(destDir, "file.txt"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeTarPath(destDir, tc.entry)
			if tc.wantErr {
				if err == nil {
					t.Errorf("sanitizeTarPath(%q, %q) = %q, want error", destDir, tc.entry, got)
				}
				return
			}
			if err != nil {
				t.Errorf("sanitizeTarPath(%q, %q) returned unexpected error: %v", destDir, tc.entry, err)
				return
			}
			if got != tc.wantPath {
				t.Errorf("sanitizeTarPath(%q, %q) = %q, want %q", destDir, tc.entry, got, tc.wantPath)
			}
		})
	}
}
