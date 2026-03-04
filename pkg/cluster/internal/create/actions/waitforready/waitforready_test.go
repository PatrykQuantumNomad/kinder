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

package waitforready

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestTryUntil_SucceedsImmediately(t *testing.T) {
	calls := 0
	result := tryUntil(context.Background(), time.Now().Add(5*time.Second), func() bool {
		calls++
		return true
	})
	if !result {
		t.Error("tryUntil should return true when try succeeds immediately")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestTryUntil_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	result := tryUntil(context.Background(), time.Now().Add(5*time.Second), func() bool {
		calls++
		return calls >= 3
	})
	if !result {
		t.Error("tryUntil should return true when try eventually succeeds")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestTryUntil_TimesOut(t *testing.T) {
	result := tryUntil(context.Background(), time.Now().Add(1*time.Second), func() bool {
		return false
	})
	if result {
		t.Error("tryUntil should return false when try never succeeds")
	}
}

func TestTryUntil_DoesNotBusyLoop(t *testing.T) {
	var calls int64
	_ = tryUntil(context.Background(), time.Now().Add(2*time.Second), func() bool {
		atomic.AddInt64(&calls, 1)
		return false
	})
	// With 500ms sleep between attempts and a 2s window, we expect
	// approximately 4 calls (at t=0, t=0.5, t=1.0, t=1.5).
	// Allow some margin but ensure it's not a busy loop (which would
	// produce thousands of calls).
	count := atomic.LoadInt64(&calls)
	if count > 5 {
		t.Errorf("tryUntil called try() %d times in 2s; expected at most ~5 with 500ms sleep (busy loop detected)", count)
	}
	if count < 2 {
		t.Errorf("tryUntil called try() only %d times in 2s; expected at least 2", count)
	}
}

func TestTryUntil_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	result := tryUntil(ctx, time.Now().Add(5*time.Second), func() bool {
		return false
	})
	if result {
		t.Error("tryUntil should return false when context is already cancelled")
	}
}
