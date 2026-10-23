// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-test-timing-parallel (run by `make semgrep`).
package fixtures

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func op() time.Duration { return time.Millisecond }

func TestParallelTiming(t *testing.T) {
	t.Parallel()
	// ruleid: bothy-go-test-timing-parallel
	_, _ = testkit.TimingSame(time.Millisecond, op, op)
}

func TestParallelSubtestTiming(t *testing.T) {
	t.Parallel()
	t.Run("sub", func(t *testing.T) {
		// ruleid: bothy-go-test-timing-parallel
		_, _ = testkit.TimingSame(time.Millisecond, op, op)
	})
}

func TestSequentialParentParallelSubtest(t *testing.T) {
	t.Run("sub", func(t *testing.T) {
		t.Parallel()
		// ruleid: bothy-go-test-timing-parallel
		_, _ = testkit.TimingSame(time.Millisecond, op, op)
	})
}

func TestSequentialTiming(t *testing.T) {
	// ok: bothy-go-test-timing-parallel
	_, _ = testkit.TimingSame(time.Millisecond, op, op)
}

func TestParallelWithoutTiming(t *testing.T) {
	t.Parallel()
	// ok: bothy-go-test-timing-parallel
	_ = testkit.TimingBound(time.Millisecond)
}
