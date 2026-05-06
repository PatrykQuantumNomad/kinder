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
	"testing"
	"time"
)

const debounceTestWindow = 50 * time.Millisecond

// countEmits drains ch for the given duration and returns how many emits
// arrived. Stops early if ch is closed.
func countEmits(ch <-chan struct{}, d time.Duration) int {
	n := 0
	deadline := time.After(d)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return n
			}
			n++
		case <-deadline:
			return n
		}
	}
}

func TestDebounce_CoalescesBurst(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := make(chan struct{}, 10)
	out := Debounce(ctx, in, debounceTestWindow)

	// Burst of 10 events with no sleep — must coalesce into exactly 1 emit.
	for i := 0; i < 10; i++ {
		in <- struct{}{}
	}

	// Wait 4x window, then count: there must be EXACTLY 1 emit.
	got := countEmits(out, 4*debounceTestWindow)
	if got != 1 {
		t.Fatalf("expected exactly 1 emit from a 10-event burst, got %d", got)
	}
}

func TestDebounce_EmitsAfterWindow(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := make(chan struct{}, 1)
	out := Debounce(ctx, in, debounceTestWindow)

	in <- struct{}{}

	// Half-window: no emit yet.
	select {
	case <-out:
		t.Fatal("emit before window expired")
	case <-time.After(debounceTestWindow / 2):
	}

	// Full-window+grace: exactly 1 emit.
	got := countEmits(out, 2*debounceTestWindow)
	if got != 1 {
		t.Fatalf("expected exactly 1 emit after window, got %d", got)
	}
}

func TestDebounce_SeparatedEventsEmitTwice(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := make(chan struct{}, 1)
	out := Debounce(ctx, in, debounceTestWindow)

	in <- struct{}{}
	if got := countEmits(out, 3*debounceTestWindow); got != 1 {
		t.Fatalf("first burst: expected 1 emit, got %d", got)
	}

	// Wait an extra window of dead-time so the debouncer fully resets.
	time.Sleep(debounceTestWindow)

	in <- struct{}{}
	if got := countEmits(out, 3*debounceTestWindow); got != 1 {
		t.Fatalf("second burst: expected 1 emit, got %d", got)
	}
}

func TestDebounce_ClosingInClosesOut(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := make(chan struct{}, 1)
	out := Debounce(ctx, in, debounceTestWindow)

	in <- struct{}{}
	// Drain the emit so out is empty before we close in.
	if got := countEmits(out, 3*debounceTestWindow); got != 1 {
		t.Fatalf("expected 1 emit before close, got %d", got)
	}

	close(in)

	if !waitForClose(t, out, 200*time.Millisecond) {
		t.Fatal("expected out channel to close within 200ms after closing in, got timeout")
	}
}

func TestDebounce_CtxCancelClosesOut(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan struct{}, 1)
	out := Debounce(ctx, in, debounceTestWindow)

	in <- struct{}{}
	cancel()

	if !waitForClose(t, out, 200*time.Millisecond) {
		t.Fatal("expected out channel to close within 200ms after ctx cancel, got timeout")
	}
}

func TestDebounce_DropsExtraWhenConsumerSlow(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := make(chan struct{}, 10)
	out := Debounce(ctx, in, debounceTestWindow)

	// First burst fills the cap=1 output buffer (consumer never drains).
	for i := 0; i < 5; i++ {
		in <- struct{}{}
	}
	time.Sleep(2 * debounceTestWindow)

	// Second burst — consumer is still slow, second emit is dropped.
	for i := 0; i < 5; i++ {
		in <- struct{}{}
	}
	time.Sleep(2 * debounceTestWindow)

	// Now drain everything: the cap=1 output means at most 1 pending emit
	// is observable. The "boolean semantics" promise holds.
	got := countEmits(out, 2*debounceTestWindow)
	if got != 1 {
		t.Fatalf("expected exactly 1 observable emit (cap=1 output), got %d", got)
	}
}
