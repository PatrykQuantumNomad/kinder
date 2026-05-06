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
	"testing"
	"time"
)

// makeInfos builds a []Info slice with deterministic CreatedAt and Size values.
// The returned slice is ordered newest-first (matching SnapshotStore.List output).
// sizes are in bytes; if empty, 1 GB is used as a default for each.
func makeInfos(names []string, now time.Time, ages []time.Duration, sizes []int64) []Info {
	infos := make([]Info, len(names))
	for i, name := range names {
		age := time.Duration(i) * time.Hour
		if i < len(ages) {
			age = ages[i]
		}
		size := int64(1 << 30) // 1 GB default
		if i < len(sizes) {
			size = sizes[i]
		}
		infos[i] = Info{
			Name:      name,
			CreatedAt: now.Add(-age), // newest-first: smallest age = newest
			Size:      size,
		}
	}
	return infos
}

// names extracts the Name field from a []Info for easier assertion.
func names(infos []Info) []string {
	out := make([]string, len(infos))
	for i, info := range infos {
		out[i] = info.Name
	}
	return out
}

// TestKeepLast verifies that KeepLast(infos, N) returns the N oldest items
// (those beyond the first N in newest-first order) as the deletion set.
func TestKeepLast(t *testing.T) {
	t.Parallel()

	now := time.Now()
	infos := makeInfos([]string{"s0", "s1", "s2", "s3", "s4"}, now, nil, nil)
	// Simulate newest-first order by setting ages proportional to index
	// (index 0 = newest, index 4 = oldest)

	toDelete := KeepLast(infos, 2)
	got := names(toDelete)
	// KeepLast(2) keeps the 2 newest (s0, s1), deletes the rest (s2, s3, s4)
	want := []string{"s2", "s3", "s4"}
	if len(got) != len(want) {
		t.Fatalf("KeepLast(2): want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("KeepLast(2)[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

// TestKeepLastExact verifies zero deletions when count == len(infos).
func TestKeepLastExact(t *testing.T) {
	t.Parallel()

	now := time.Now()
	infos := makeInfos([]string{"s0", "s1", "s2"}, now, nil, nil)
	toDelete := KeepLast(infos, 3)
	if len(toDelete) != 0 {
		t.Errorf("KeepLast(3) on 3 snapshots: expected 0 deletions, got %d", len(toDelete))
	}
}

// TestKeepLastZero verifies that KeepLast(infos, 0) is treated as "keep nothing"
// (delete everything).
func TestKeepLastZero(t *testing.T) {
	t.Parallel()

	now := time.Now()
	infos := makeInfos([]string{"s0", "s1"}, now, nil, nil)
	toDelete := KeepLast(infos, 0)
	if len(toDelete) != 2 {
		t.Errorf("KeepLast(0): expected 2 deletions, got %d", len(toDelete))
	}
}

// TestOlderThan verifies that OlderThan deletes snapshots older than d relative
// to now.
func TestOlderThan(t *testing.T) {
	t.Parallel()

	now := time.Now()
	// Ages from newest to oldest: 1h, 2h, 25h, 49h
	ages := []time.Duration{1 * time.Hour, 2 * time.Hour, 25 * time.Hour, 49 * time.Hour}
	infos := makeInfos([]string{"s0", "s1", "s2", "s3"}, now, ages, nil)

	toDelete := OlderThan(infos, 24*time.Hour, now)
	got := names(toDelete)
	// s2 (25h) and s3 (49h) exceed 24h threshold
	want := []string{"s2", "s3"}
	if len(got) != len(want) {
		t.Fatalf("OlderThan(24h): want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("OlderThan(24h)[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

// TestMaxSize verifies that MaxSize removes oldest snapshots until total Size ≤ max.
// Sizes (newest→oldest): 1GB, 2GB, 1GB, 500MB. Total = 4.5GB. Max = 3GB.
// Delete s3(500MB)? Total = 4GB > 3GB. Delete s2(1GB)? Total = 3GB ≤ 3GB → stop.
func TestMaxSize(t *testing.T) {
	t.Parallel()

	now := time.Now()
	GB := int64(1 << 30)
	sizes := []int64{1 * GB, 2 * GB, 1 * GB, GB / 2}
	infos := makeInfos([]string{"s0", "s1", "s2", "s3"}, now, nil, sizes)

	toDelete := MaxSize(infos, 3*GB)
	got := names(toDelete)
	// Total: 4.5 GB. Delete from oldest (s3=0.5GB → total 4GB). Still > 3GB.
	// Delete s2 (1GB → total 3GB). 3GB ≤ 3GB → stop. Deleted = {s3, s2}.
	want := []string{"s2", "s3"}
	if len(got) != len(want) {
		t.Fatalf("MaxSize(3GB): want deleted %v, got %v", want, got)
	}
	// Order in deletion set: preserve newest-first order of the originals
	// (s2 comes before s3 in original slice)
	for i, w := range want {
		if got[i] != w {
			t.Errorf("MaxSize(3GB)[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

// TestMaxSizeNoDeletion verifies no deletions when total ≤ max.
func TestMaxSizeNoDeletion(t *testing.T) {
	t.Parallel()

	now := time.Now()
	GB := int64(1 << 30)
	sizes := []int64{500 * 1024 * 1024, 500 * 1024 * 1024} // 500MB each = 1GB total
	infos := makeInfos([]string{"s0", "s1"}, now, nil, sizes)

	toDelete := MaxSize(infos, 3*GB)
	if len(toDelete) != 0 {
		t.Errorf("MaxSize(3GB) with 1GB total: expected 0 deletions, got %d", len(toDelete))
	}
}

// TestPrunePolicies verifies table-driven scenarios for KeepLast, OlderThan,
// MaxSize, and combined PrunePlan with union semantics.
func TestPrunePolicies(t *testing.T) {
	t.Parallel()

	now := time.Now()
	GB := int64(1 << 30)

	tests := []struct {
		name        string
		infos       []Info
		policy      Policy
		wantDeleted []string
	}{
		{
			name: "KeepLastN only",
			infos: makeInfos(
				[]string{"s0", "s1", "s2", "s3", "s4"},
				now, nil, nil,
			),
			policy:      Policy{KeepLastN: 3},
			wantDeleted: []string{"s3", "s4"},
		},
		{
			name: "OlderThan only",
			infos: makeInfos(
				[]string{"s0", "s1", "s2", "s3"},
				now,
				[]time.Duration{1 * time.Hour, 2 * time.Hour, 25 * time.Hour, 49 * time.Hour},
				nil,
			),
			policy:      Policy{OlderThan: 24 * time.Hour},
			wantDeleted: []string{"s2", "s3"},
		},
		{
			name: "MaxSize only",
			infos: makeInfos(
				[]string{"s0", "s1", "s2", "s3"},
				now, nil,
				[]int64{1 * GB, 2 * GB, 1 * GB, GB / 2},
			),
			policy:      Policy{MaxSize: 3 * GB},
			wantDeleted: []string{"s2", "s3"},
		},
		{
			// Union semantics: s3 and s4 are past KeepLastN=3 threshold;
			// s2 and s3 are older than 10d (ages 12d and 15d);
			// MaxSize=10G is generous enough not to trigger.
			// Union → s2, s3, s4 deleted.
			name: "combined KeepLastN+OlderThan+MaxSize union",
			infos: makeInfos(
				[]string{"s0", "s1", "s2", "s3", "s4"},
				now,
				[]time.Duration{
					1 * 24 * time.Hour,
					5 * 24 * time.Hour,
					12 * 24 * time.Hour,
					15 * 24 * time.Hour,
					20 * 24 * time.Hour,
				},
				[]int64{1 * GB, 1 * GB, 1 * GB, 1 * GB, 1 * GB},
			),
			policy: Policy{
				KeepLastN: 3,
				OlderThan: 10 * 24 * time.Hour,
				MaxSize:   10 * GB,
			},
			// KeepLastN=3 → delete s3, s4 (indexes 3,4)
			// OlderThan=10d → delete s2(12d), s3(15d), s4(20d)
			// MaxSize=10G → total=5GB ≤ 10GB → delete nothing
			// Union: s2, s3, s4 deleted
			wantDeleted: []string{"s2", "s3", "s4"},
		},
		{
			name:        "empty input",
			infos:       []Info{},
			policy:      Policy{KeepLastN: 3},
			wantDeleted: []string{},
		},
		{
			name: "all policies zero → no deletions",
			infos: makeInfos(
				[]string{"s0", "s1"},
				now, nil, nil,
			),
			policy:      Policy{},
			wantDeleted: []string{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			toDelete := PrunePlan(tc.infos, tc.policy, now)
			got := names(toDelete)

			// Build sets for comparison (order within deletion set is preserved from
			// input order, but we check by set membership for combined cases).
			wantSet := make(map[string]bool, len(tc.wantDeleted))
			for _, w := range tc.wantDeleted {
				wantSet[w] = true
			}
			gotSet := make(map[string]bool, len(got))
			for _, g := range got {
				gotSet[g] = true
			}

			if len(gotSet) != len(wantSet) {
				t.Errorf("PrunePlan deletion set length: want %d (%v), got %d (%v)",
					len(wantSet), tc.wantDeleted, len(gotSet), got)
				return
			}
			for w := range wantSet {
				if !gotSet[w] {
					t.Errorf("PrunePlan: expected %q in deletion set, not found; got %v", w, got)
				}
			}
		})
	}
}
