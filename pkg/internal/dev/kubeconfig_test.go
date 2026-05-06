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

package dev

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster"
)

// withKubeconfigGetter swaps the package-level getter for the duration of t.
func withKubeconfigGetter(t *testing.T, fn func(p *cluster.Provider, name string, internal bool) (string, error)) {
	t.Helper()
	prev := kubeconfigGetter
	kubeconfigGetter = fn
	t.Cleanup(func() { kubeconfigGetter = prev })
}

// countKinderDevTempFiles returns the number of files in os.TempDir() matching
// the kinder-dev-*.kubeconfig pattern. Used to assert no leftover temp files
// on error paths.
func countKinderDevTempFiles(t *testing.T) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "kinder-dev-*.kubeconfig"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return len(matches)
}

func TestWriteKubeconfigTemp_Mode0600(t *testing.T) {
	withKubeconfigGetter(t, func(_ *cluster.Provider, _ string, _ bool) (string, error) {
		return "test-kubeconfig-content", nil
	})

	path, cleanup, err := WriteKubeconfigTemp(nil, "any-cluster")
	if err != nil {
		t.Fatalf("WriteKubeconfigTemp returned error: %v", err)
	}
	if path == "" {
		t.Fatal("WriteKubeconfigTemp returned empty path")
	}
	if cleanup == nil {
		t.Fatal("WriteKubeconfigTemp returned nil cleanup")
	}
	t.Cleanup(cleanup)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kubeconfig file: %v", err)
	}
	if string(got) != "test-kubeconfig-content" {
		t.Errorf("content = %q, want %q", string(got), "test-kubeconfig-content")
	}

	// mode 0600 (security V4) — verified via os.Stat. Skip on Windows where
	// Unix mode bits are not enforced the same way.
	if runtime.GOOS == "windows" {
		t.Skip("skipping mode bit assertion on Windows")
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0600 {
		t.Errorf("mode = %o, want %o (V4 mitigation: kubeconfig contains client cert/key)", got, 0600)
	}

	// path naming convention — must match the kinder-dev-*.kubeconfig pattern
	// so concurrent kinder dev invocations don't clobber.
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "kinder-dev-") || !strings.HasSuffix(base, ".kubeconfig") {
		t.Errorf("path basename %q does not match kinder-dev-*.kubeconfig", base)
	}
}

func TestWriteKubeconfigTemp_CleanupRemoves(t *testing.T) {
	withKubeconfigGetter(t, func(_ *cluster.Provider, _ string, _ bool) (string, error) {
		return "x", nil
	})
	path, cleanup, err := WriteKubeconfigTemp(nil, "c")
	if err != nil {
		t.Fatalf("WriteKubeconfigTemp: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist before cleanup: %v", err)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed after cleanup; stat err = %v", err)
	}
}

func TestWriteKubeconfigTemp_CleanupIsIdempotent(t *testing.T) {
	withKubeconfigGetter(t, func(_ *cluster.Provider, _ string, _ bool) (string, error) {
		return "x", nil
	})
	_, cleanup, err := WriteKubeconfigTemp(nil, "c")
	if err != nil {
		t.Fatalf("WriteKubeconfigTemp: %v", err)
	}
	// Second call must not panic. os.Remove on a missing file returns
	// *PathError which the cleanup func swallows.
	cleanup()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("second cleanup call panicked: %v", r)
		}
	}()
	cleanup()
}

func TestWriteKubeconfigTemp_PropagatesProviderError(t *testing.T) {
	before := countKinderDevTempFiles(t)
	withKubeconfigGetter(t, func(_ *cluster.Provider, _ string, _ bool) (string, error) {
		return "", errors.New("boom from provider")
	})

	path, cleanup, err := WriteKubeconfigTemp(nil, "c")
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("expected error from WriteKubeconfigTemp, got nil")
	}
	if path != "" {
		t.Errorf("expected empty path on error, got %q", path)
	}
	if cleanup != nil {
		t.Errorf("expected nil cleanup on error, got non-nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "boom from provider") {
		t.Errorf("error %q should contain underlying provider error", msg)
	}
	if !strings.Contains(msg, "kubeconfig") {
		t.Errorf("error %q should mention kubeconfig for context", msg)
	}

	// Critical: no leftover temp file on error path.
	after := countKinderDevTempFiles(t)
	if after != before {
		t.Errorf("temp file leak: had %d kinder-dev-*.kubeconfig files before, %d after error path", before, after)
	}
}
