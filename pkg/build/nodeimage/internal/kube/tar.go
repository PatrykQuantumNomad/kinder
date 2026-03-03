package kube

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kind/pkg/log"
)

// sanitizeTarPath validates that the tar entry name does not escape the
// destination directory (prevents zip-slip / path-traversal attacks).
func sanitizeTarPath(destDirectory, name string) (string, error) {
	// Reject absolute paths outright -- tar entries should always be relative.
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("illegal file path in tar: %s", name)
	}
	destPath := filepath.Join(destDirectory, name)
	if !strings.HasPrefix(filepath.Clean(destPath)+string(os.PathSeparator), filepath.Clean(destDirectory)+string(os.PathSeparator)) &&
		filepath.Clean(destPath) != filepath.Clean(destDirectory) {
		return "", fmt.Errorf("illegal file path in tar: %s", name)
	}
	return destPath, nil
}

// extractTarball takes a gzipped-tarball and extracts the contents into a specified directory
func extractTarball(tarPath, destDirectory string, logger log.Logger) (err error) {
	// Open the tar file
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("opening tarball: %w", err)
	}
	defer f.Close()

	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gzipReader)

	numFiles := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tarfile %s: %w", tarPath, err)
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		dirPath, err := sanitizeTarPath(destDirectory, filepath.Dir(hdr.Name))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dirPath, os.FileMode(0o755)); err != nil {
			return fmt.Errorf("creating image directory structure: %w", err)
		}

		filePath, err := sanitizeTarPath(destDirectory, hdr.Name)
		if err != nil {
			return err
		}
		f, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("creating image layer file: %w", err)
		}

		if _, err := io.CopyN(f, tr, hdr.Size); err != nil {
			f.Close()
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return fmt.Errorf("archive truncated: unexpected EOF while extracting %s", hdr.Name)
			}

			return fmt.Errorf("extracting image data: %w", err)
		}
		f.Close()

		numFiles++
	}

	logger.V(2).Infof("Successfully extracted %d files from image tarball %s", numFiles, tarPath)
	return err
}
