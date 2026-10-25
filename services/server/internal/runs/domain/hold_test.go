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

// TestHoldWindow (M3-09 design note 13): a message held only for its pending transcript waits at
// most voice.transcript_hold_max, every other hold (an attachment not final) at most
// file.invocation_hold_max.
func TestHoldWindow(t *testing.T) {
	t.Parallel()
	fileMax, voiceMax := 2*time.Minute, 5*time.Minute
	if got := domain.HoldWindow(true, fileMax, voiceMax); got != voiceMax {
		t.Fatalf("transcript only: %v, want voice.transcript_hold_max %v", got, voiceMax)
	}
	if got := domain.HoldWindow(false, fileMax, voiceMax); got != fileMax {
		t.Fatalf("files pending: %v, want file.invocation_hold_max %v", got, fileMax)
	}
}

// TestEffectiveVoiceInput (09 section 2.2, R-AG-01, M3-09): voice_input is effective only when the
// owner does not refuse supports_voice (nil allows) and the adapter reported voice_input.
func TestEffectiveVoiceInput(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	for _, c := range []struct {
		permitted *bool
		reported  bool
		want      bool
	}{
		{nil, true, true}, {&yes, true, true}, {&no, true, false}, {nil, false, false}, {&yes, false, false}, {&no, false, false},
	} {
		if got := domain.EffectiveVoiceInput(c.permitted, c.reported); got != c.want {
			t.Errorf("permitted %v reported %v: %v, want %v", c.permitted, c.reported, got, c.want)
		}
	}
}
