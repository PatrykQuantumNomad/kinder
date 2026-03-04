/*
Copyright 2019 The Kubernetes Authors.

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

package actions_test

import (
	"sync"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/testutil"
	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// TestNodes_ConcurrentAccess verifies that ActionContext.Nodes() is
// race-free when called concurrently from multiple goroutines.
func TestNodes_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	cp := testutil.NewFakeControlPlane("cp", nil)
	provider := &testutil.FakeProvider{
		Nodes:    []nodes.Node{cp},
		InfoResp: &providers.ProviderInfo{},
	}
	ctx := testutil.NewTestContext(provider)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := ctx.Nodes()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Errorf("expected 1 node, got %d", len(got))
			}
		}()
	}
	wg.Wait()
}

// TestNodes_CachesResult verifies that subsequent calls to Nodes()
// return the same cached result without re-calling ListNodes.
func TestNodes_CachesResult(t *testing.T) {
	t.Parallel()
	cp := testutil.NewFakeControlPlane("cp", nil)
	provider := &testutil.FakeProvider{
		Nodes:    []nodes.Node{cp},
		InfoResp: &providers.ProviderInfo{},
	}
	ctx := testutil.NewTestContext(provider)

	first, err := ctx.Nodes()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := ctx.Nodes()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if &first[0] != &second[0] {
		t.Error("expected Nodes() to return cached result on second call")
	}
}
