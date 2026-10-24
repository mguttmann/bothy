// SPDX-License-Identifier: Apache-2.0

package searchworld

import (
	"context"
	"errors"
	"testing"
)

// TestParallelReportsCancellation: a context that ends before every call started gives its error,
// so Seed and Refresh never report success for work they skipped; all calls done gives nil.
func TestParallelReportsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	calls := 0
	err := parallel(ctx, 5, func(context.Context, int) error { calls++; return nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancelled: err %v after %d calls, want context.Canceled and no call", err, calls)
	}
	done := make([]bool, 40)
	if err := parallel(t.Context(), len(done), func(_ context.Context, i int) error { done[i] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	for i, ok := range done {
		if !ok {
			t.Fatalf("call %d did not run", i)
		}
	}
}
