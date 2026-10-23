// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestHoldUntil: not pending gives the zero time (not held); pending gives the given clock reading
// plus file.invocation_hold_max.
func TestHoldUntil(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	holdMax := 3 * time.Minute
	if got := domain.HoldUntil(now, false, holdMax); !got.IsZero() {
		t.Fatalf("not pending: %v, want zero", got)
	}
	if got := domain.HoldUntil(now, true, holdMax); !got.Equal(now.Add(holdMax)) {
		t.Fatalf("pending: %v, want %v", got, now.Add(holdMax))
	}
}

// TestReleaseVerdict walks state times held times cause times pending: only a queued, held run is
// released, for ReleaseFinal only when nothing is pending, for ReleaseTimeout regardless.
func TestReleaseVerdict(t *testing.T) {
	t.Parallel()
	held := time.Date(2026, 10, 21, 9, 3, 0, 0, time.UTC)
	for _, st := range domain.States() {
		for _, h := range []time.Time{{}, held} {
			for _, cause := range []domain.ReleaseCause{domain.ReleaseFinal, domain.ReleaseTimeout} {
				for _, pending := range []bool{false, true} {
					want := st == domain.StateQueued && !h.IsZero() && (cause == domain.ReleaseTimeout || !pending)
					if got := domain.ReleaseVerdict(st, h, cause, pending); got != want {
						t.Errorf("ReleaseVerdict(%s, held %v, %v, pending %v) = %v, want %v", st, !h.IsZero(), cause, pending, got, want)
					}
				}
			}
		}
	}
	if domain.ReleaseVerdict(domain.StateQueued, held, domain.ReleaseCause(0), false) {
		t.Fatal("an unknown cause released")
	}
}
