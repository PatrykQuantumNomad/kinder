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
	"context"
	"time"
)

// Debounce coalesces rapid sends on `in` into single emits on the
// returned channel: at most one emit per `window`. The first send on
// `in` arms a timer; subsequent sends within the window do NOT extend
// the window (leading-trigger debounce: "fire `window` after the FIRST
// event in a burst"). When the timer expires, exactly one emit is sent
// on the output (non-blocking — if the consumer hasn't drained the
// previous emit, the new emit is dropped, since the consumer already
// knows "something changed").
//
// The output channel is closed when `in` is closed OR when ctx is
// canceled. Output buffer is cap=1 — the consumer only ever needs to
// know "something changed since you last drained" (boolean semantics).
//
// Why "first-event arms" not "last-event resets":
// Trailing-edge debounce (last event resets) is what fast-typing UIs
// want — wait until the user STOPS typing. For file-save bursts (vim
// swap-file rename, IDE atomic write), the burst is 5–50 events over
// <100ms; we want to fire `window` after the FIRST event so the
// build-load-rollout cycle starts as early as possible. RESEARCH §6
// shows this pattern in the "channel-based debounce" variant.
func Debounce(ctx context.Context, in <-chan struct{}, window time.Duration) <-chan struct{} {
	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		// time.NewTimer(d) returns an armed timer. Stop + drain so the first
		// loop iteration sees a quiescent timer; otherwise a stale fire could
		// trip case <-timer.C before any event arrives.
		timer := time.NewTimer(window)
		if !timer.Stop() {
			<-timer.C
		}

		pending := false
		for {
			select {
			case _, ok := <-in:
				if !ok {
					return
				}
				if !pending {
					timer.Reset(window)
					pending = true
				}
				// While pending, additional sends are intentionally absorbed —
				// the timer continues running from the FIRST send.
			case <-timer.C:
				pending = false
				select {
				case out <- struct{}{}:
				default:
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
