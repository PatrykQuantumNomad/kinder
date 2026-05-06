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
	"fmt"
	"os"

	"sigs.k8s.io/kind/pkg/cluster"
)

// kubeconfigGetter is the indirection point used by tests to substitute a
// canned kubeconfig string for `provider.KubeConfig(name, internal)`. In
// production it forwards to the real Provider method.
//
// We swap the function rather than the *cluster.Provider because building a
// fake *cluster.Provider would require a real internal provider — way more
// machinery than this primitive needs.
var kubeconfigGetter = func(p *cluster.Provider, name string, internal bool) (string, error) {
	return p.KubeConfig(name, internal)
}

// WriteKubeconfigTemp obtains the cluster's kubeconfig (using the
// host-reachable EXTERNAL endpoint per RESEARCH A4: provider.KubeConfig
// takes an `internal bool` parameter where false = NOT internal = external),
// writes it to a unique temp file with mode 0600 (kubeconfigs contain
// client cert+key — they ARE credentials, RESEARCH security V4), and
// returns the path along with a cleanup function the caller MUST defer.
//
// On any error, no file is left behind and cleanup is nil. The cleanup is
// safe to call multiple times (os.Remove on a missing file returns an
// error but does not panic, which we swallow).
//
// Why os.CreateTemp + Chmod rather than os.WriteFile(path, content, 0600):
// os.WriteFile takes a fixed path; we need a unique path so concurrent
// `kinder dev` invocations against different clusters do not clobber each
// other's temp file.
func WriteKubeconfigTemp(provider *cluster.Provider, clusterName string) (path string, cleanup func(), err error) {
	cfg, err := kubeconfigGetter(provider, clusterName, false)
	if err != nil {
		return "", nil, fmt.Errorf("get kubeconfig for cluster %q: %w", clusterName, err)
	}

	f, err := os.CreateTemp("", "kinder-dev-*.kubeconfig")
	if err != nil {
		return "", nil, fmt.Errorf("create kubeconfig tempfile: %w", err)
	}

	// Tighten permissions BEFORE writing — even though CreateTemp creates
	// 0600 on Unix, be explicit (V4 mitigation; defensive against unusual
	// umask configurations).
	if chmodErr := os.Chmod(f.Name(), 0600); chmodErr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("chmod kubeconfig: %w", chmodErr)
	}

	if _, writeErr := f.WriteString(cfg); writeErr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("write kubeconfig: %w", writeErr)
	}

	if closeErr := f.Close(); closeErr != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("close kubeconfig: %w", closeErr)
	}

	name := f.Name()
	cleanup = func() {
		// Idempotent: ignore the error from a second/post-cleanup call.
		_ = os.Remove(name)
	}
	return name, cleanup, nil
}
