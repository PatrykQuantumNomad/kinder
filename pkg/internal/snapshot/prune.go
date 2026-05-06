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
	"time"
)

// Policy describes the retention rules for PrunePlan. A zero-value Policy
// produces no deletions. Policies compose as a UNION (OR): a snapshot is
// deleted if it would be deleted by ANY active policy. An "active" policy field
// is one with a non-zero value (KeepLastN > 0, OlderThan > 0, MaxSize > 0).
//
// The CLI command `kinder snapshot prune` enforces that at least one policy
// flag is set before calling PrunePlan — this pure function itself does not
// enforce that requirement.
type Policy struct {
	// KeepLastN: keep the N newest snapshots; delete the rest.
	// 0 = policy not active (no deletions for this field).
	KeepLastN int
	// OlderThan: delete snapshots older than this duration relative to now.
	// 0 = policy not active.
	OlderThan time.Duration
	// MaxSize: delete oldest snapshots until total size (sum of all Info.Size)
	// is at or below this threshold in bytes.
	// 0 = policy not active.
	MaxSize int64
}

// PrunePlan returns the subset of infos to delete to satisfy policy. The input
// must be sorted newest-first (matching SnapshotStore.List output). Returned
// slice preserves the original newest-first ordering among deleted items.
// All active policy fields are combined as a UNION: a snapshot appears in the
// deletion set if ANY active policy marks it for deletion.
func PrunePlan(infos []Info, policy Policy, now time.Time) []Info {
	if len(infos) == 0 {
		return nil
	}

	// Build a set of indices to delete.
	toDelete := make(map[int]bool, len(infos))

	if policy.KeepLastN > 0 {
		for _, info := range KeepLast(infos, policy.KeepLastN) {
			for i := range infos {
				if infos[i].Name == info.Name {
					toDelete[i] = true
				}
			}
		}
	}

	if policy.OlderThan > 0 {
		for _, info := range OlderThan(infos, policy.OlderThan, now) {
			for i := range infos {
				if infos[i].Name == info.Name {
					toDelete[i] = true
				}
			}
		}
	}

	if policy.MaxSize > 0 {
		for _, info := range MaxSize(infos, policy.MaxSize) {
			for i := range infos {
				if infos[i].Name == info.Name {
					toDelete[i] = true
				}
			}
		}
	}

	if len(toDelete) == 0 {
		return nil
	}

	// Collect in original (newest-first) order.
	result := make([]Info, 0, len(toDelete))
	for i, info := range infos {
		if toDelete[i] {
			result = append(result, info)
		}
	}
	return result
}

// KeepLast returns the snapshots that would be deleted to keep the newest n.
// Infos must be sorted newest-first. If n >= len(infos), no deletions occur.
// If n <= 0, all snapshots are returned (delete everything).
func KeepLast(infos []Info, n int) []Info {
	if n < 0 {
		n = 0
	}
	if n >= len(infos) {
		return nil
	}
	return infos[n:]
}

// OlderThan returns snapshots where now.Sub(info.CreatedAt) > d.
// Infos must be sorted newest-first; returned slice preserves that order.
func OlderThan(infos []Info, d time.Duration, now time.Time) []Info {
	var result []Info
	for _, info := range infos {
		if now.Sub(info.CreatedAt) > d {
			result = append(result, info)
		}
	}
	return result
}

// MaxSize returns the snapshots that should be deleted to bring the total size
// of all infos to at or below max bytes. Deletion starts from the oldest
// snapshot (last in newest-first order) and proceeds toward the newest until
// the constraint is satisfied.
// Infos must be sorted newest-first; returned slice preserves newest-first
// ordering among deleted items.
func MaxSize(infos []Info, max int64) []Info {
	if len(infos) == 0 {
		return nil
	}

	// Compute total size.
	var total int64
	for _, info := range infos {
		total += info.Size
	}

	if total <= max {
		return nil
	}

	// Walk from oldest (tail) toward newest, collecting deletions until we fit.
	// We build the set from the tail and then reverse to restore newest-first order.
	var deleted []int
	for i := len(infos) - 1; i >= 0 && total > max; i-- {
		total -= infos[i].Size
		deleted = append(deleted, i)
	}

	// Reverse so the result is in newest-first order (smallest index first).
	result := make([]Info, len(deleted))
	for j, idx := range deleted {
		result[len(deleted)-1-j] = infos[idx]
	}
	return result
}
