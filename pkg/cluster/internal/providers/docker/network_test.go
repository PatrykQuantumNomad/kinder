/*
Copyright 2020 The Kubernetes Authors.

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

package docker

import (
	"fmt"
	"reflect"
	"testing"

	"sigs.k8s.io/kind/pkg/internal/assert"
)

func Test_generateULASubnetFromName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		attempt int32
		subnet  string
	}{
		{
			name:   "kind",
			subnet: "fc00:4d57:1afd:1f1b::/64",
		},
		{
			name:    "foo",
			attempt: 1,
			subnet:  "fc00:3b86:d0fc:521c::/64",
		},
		{
			name:    "foo",
			attempt: 2,
			subnet:  "fc00:2774:110d:111a::/64",
		},
		{
			name:   "kind2",
			subnet: "fc00:724:8e67:eb33::/64",
		},
		{
			name:   "kin",
			subnet: "fc00:34fb:893a:fe4a::/64",
		},
		{
			name:   "mysupernetwork",
			subnet: "fc00:1567:8728:937b::/64",
		},
	}
	for _, tc := range cases {
		tc := tc // capture variable
		t.Run(fmt.Sprintf("%s,%d", tc.name, tc.attempt), func(t *testing.T) {
			t.Parallel()
			subnet := generateULASubnetFromName(tc.name, tc.attempt)
			if subnet != tc.subnet {
				t.Errorf("Wrong subnet from %v: expected %v, received %v", tc.name, tc.subnet, subnet)
			}
		})
	}
}

func Test_sortNetworkInspectEntries(t *testing.T) {
	cases := []struct {
		Name     string
		Networks []networkInspectEntry
		Sorted   []networkInspectEntry
	}{
		{
			Name: "simple ID sort",
			Networks: []networkInspectEntry{
				{
					ID: "dc7f897c237215c3b73d2c9ba1d4e116d872793a6c1c0e5bf083762998de8b4e",
				},
				{
					ID: "1ed9912325a0d08594ee786de91ebd961e631643877b5ee58ec906b640813eae",
				},
			},
			Sorted: []networkInspectEntry{
				{
					ID: "1ed9912325a0d08594ee786de91ebd961e631643877b5ee58ec906b640813eae",
				},
				{
					ID: "dc7f897c237215c3b73d2c9ba1d4e116d872793a6c1c0e5bf083762998de8b4e",
				},
			},
		},
		{
			// BUG-04: network with more containers must sort first even when its ID
			// is lexicographically greater than the network with fewer containers.
			// The buggy comparator evaluates less(zzzz, aaaa): 1 > 0 = true, so it
			// places zzzz before aaaa during that comparison. But less(aaaa, zzzz):
			// 0 > 1 = false, falls through to "aaaa" < "zzzz" = true — meaning aaaa
			// is also "less than" zzzz. This violates strict weak ordering and causes
			// sort.Slice to place aaaa first when zzzz appears first in the input.
			Name: "more containers wins over lower ID",
			Networks: []networkInspectEntry{
				{
					// zzzz listed first, 1 container — should still sort first after fix
					ID: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
					Containers: map[string]map[string]string{
						"c1": {"Name": "node1"},
					},
				},
				{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			},
			Sorted: []networkInspectEntry{
				{
					ID: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
					Containers: map[string]map[string]string{
						"c1": {"Name": "node1"},
					},
				},
				{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			},
		},
		{
			Name: "containers attached sort",
			Networks: []networkInspectEntry{
				{
					ID: "1ed9912325a0d08594ee786de91ebd961e631643877b5ee58ec906b640813eae",
				},
				{
					ID: "dc7f897c237215c3b73d2c9ba1d4e116d872793a6c1c0e5bf083762998de8b4e",
					Containers: map[string]map[string]string{
						"a37779e06f3b694eba491dd450aad18bbbaa0a0fce2952e7c9195ea45ae79d41": {
							"Name":       "buildx_buildkit_kind-builder0",
							"EndpointID": "8f6411fb4360059b2f91028f91ef03130abc96d6381afc265ce53c9df89d5a3d",
						},
					},
				},
				{
					ID: "f0445f08b9989921da00250d778975202267fbab364e5fbad0ceb6db24f3f91e",
				},
				{
					ID: "128154205c7d88c7bb9c255d389bc9e222b58a48cf83619976e7665a48e79918",
					Containers: map[string]map[string]string{
						"aad18bbbaa0a0fce2952e7c9195ea45ae79d41a37779e06f3b694eba491dd450": {
							"Name":       "fakey-fake",
							"EndpointID": "f03130abc96d6381afc265ce53c9df89d5a3d8f6411fb4360059b2f91028f91e",
						},
					},
				},
			},
			Sorted: []networkInspectEntry{
				{
					ID: "128154205c7d88c7bb9c255d389bc9e222b58a48cf83619976e7665a48e79918",
					Containers: map[string]map[string]string{
						"aad18bbbaa0a0fce2952e7c9195ea45ae79d41a37779e06f3b694eba491dd450": {
							"Name":       "fakey-fake",
							"EndpointID": "f03130abc96d6381afc265ce53c9df89d5a3d8f6411fb4360059b2f91028f91e",
						},
					},
				},
				{
					ID: "dc7f897c237215c3b73d2c9ba1d4e116d872793a6c1c0e5bf083762998de8b4e",
					Containers: map[string]map[string]string{
						"a37779e06f3b694eba491dd450aad18bbbaa0a0fce2952e7c9195ea45ae79d41": {
							"Name":       "buildx_buildkit_kind-builder0",
							"EndpointID": "8f6411fb4360059b2f91028f91ef03130abc96d6381afc265ce53c9df89d5a3d",
						},
					},
				},
				{
					ID: "1ed9912325a0d08594ee786de91ebd961e631643877b5ee58ec906b640813eae",
				},
				{
					ID: "f0445f08b9989921da00250d778975202267fbab364e5fbad0ceb6db24f3f91e",
				},
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			toSort := make([]networkInspectEntry, len(tc.Networks))
			copy(toSort, tc.Networks)
			sortNetworkInspectEntries(toSort)
			assert.DeepEqual(t, tc.Sorted, toSort)
		})
	}
}

// Test_sortNetworkInspectEntries_Deterministic verifies that repeated sorting
// of the same input always produces identical output — a requirement for stable
// duplicate-network removal. A non-deterministic sort could pick different
// "winner" networks across runs.
func Test_sortNetworkInspectEntries_Deterministic(t *testing.T) {
	t.Parallel()

	input := []networkInspectEntry{
		{ID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
		{
			ID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Containers: map[string]map[string]string{
				"c1": {"Name": "node-a"},
			},
		},
		{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{
			ID: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			Containers: map[string]map[string]string{
				"c2": {"Name": "node-b"},
			},
		},
		{ID: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
	}

	// Perform the first sort to establish the canonical result.
	first := make([]networkInspectEntry, len(input))
	copy(first, input)
	sortNetworkInspectEntries(first)

	// Sort the same input 99 more times and confirm the result is identical.
	for i := 0; i < 99; i++ {
		run := make([]networkInspectEntry, len(input))
		copy(run, input)
		sortNetworkInspectEntries(run)
		if !reflect.DeepEqual(first, run) {
			t.Errorf("sort was non-deterministic: run %d produced different order", i+2)
			return
		}
	}
}
